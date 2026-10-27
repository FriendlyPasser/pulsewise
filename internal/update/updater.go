package update

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Every is how often the feed is asked by itself.
const Every = 6 * time.Hour

// State is where an update stands, as the panel shows it.
type State string

const (
	Off         State = "off"         // not a release: never checks
	Idle        State = "idle"        // not checked yet
	Checking    State = "checking"    //
	Current     State = "current"     // the latest is the one running
	Downloading State = "downloading" //
	Ready       State = "ready"       // downloaded: a restart puts it in
	Available   State = "available"   // newer, but this app can't replace itself (Stuck)
	Failed      State = "error"       //
)

// Status is the updater's state, for the panel.
type Status struct {
	State   State  `json:"state"`
	Current string `json:"current,omitempty"`
	Latest  string `json:"latest,omitempty"`
	Notes   string `json:"notes,omitempty"`
	URL     string `json:"url,omitempty"`
	Stuck   string `json:"stuck,omitempty"`
	Error   string `json:"error,omitempty"`
	Done    int64  `json:"done,omitempty"`
	Total   int64  `json:"total,omitempty"`
}

// Updater keeps the app at bundle current: it asks the feed at start-up
// and every Every, downloads a newer release straight away, and leaves it
// staged for Restart, or for Quit to put in.
type Updater struct {
	bundle  string
	current string
	stuck   string

	mu      sync.Mutex
	state   State
	latest  *Release
	err     string
	staged  string
	done    int64
	total   int64
	asked   time.Time
	onReady func(version string)
}

// New is the updater of the running app. It is Off when the app isn't a
// release: not in a bundle, or with no version or signing team.
func New() *Updater {
	u := &Updater{state: Off}
	b := Bundle()
	if b == "" {
		return u
	}
	v := Version(b)
	if parse(v) == nil || Team(context.Background(), b) == "" {
		return u
	}
	u.bundle, u.current, u.stuck, u.state = b, v, Stuck(b), Idle
	return u
}

// OnReady has f called, on its own goroutine, when a version is staged.
func (u *Updater) OnReady(f func(version string)) {
	u.mu.Lock()
	u.onReady = f
	u.mu.Unlock()
}

// Start checks a few seconds from now and then every Every. The clock is
// the wall's, looked at each minute, so a Mac asleep through the interval
// checks when it wakes.
func (u *Updater) Start() {
	if u.state == Off {
		return
	}
	go func() {
		time.Sleep(5 * time.Second)
		for {
			u.mu.Lock()
			due := time.Now().Round(0).Sub(u.asked) >= Every
			u.mu.Unlock()
			if due {
				u.Check()
			}
			time.Sleep(time.Minute)
		}
	}()
}

// Check asks the feed now, unless a check or a download is under way, and
// downloads what is newer. It returns at once.
func (u *Updater) Check() {
	u.mu.Lock()
	if u.state == Off || u.state == Checking || u.state == Downloading {
		u.mu.Unlock()
		return
	}
	u.state, u.err, u.asked = Checking, "", time.Now().Round(0)
	u.mu.Unlock()
	go u.run()
}

func (u *Updater) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rel, err := Latest(ctx)

	u.mu.Lock()
	defer u.mu.Unlock()
	switch {
	case err != nil && u.staged != "":
		u.state = Ready // what was downloaded is still there to put in
		return
	case err != nil:
		u.state, u.err = Failed, err.Error()
		return
	case !Newer(rel.Version, u.current):
		u.state, u.latest = Current, rel
		return
	case u.staged != "" && !Newer(rel.Version, u.latest.Version):
		u.state = Ready
		return
	case u.stuck != "":
		u.state, u.latest = Available, rel
		return
	}

	u.latest, u.staged, u.done, u.total, u.state = rel, "", 0, 0, Downloading
	u.mu.Unlock()
	staged, err := Stage(ctx, rel, u.bundle, func(done, total int64) {
		u.mu.Lock()
		u.done, u.total = done, total
		u.mu.Unlock()
	})
	u.mu.Lock()
	if err != nil {
		log.Println("pulsewise: update:", err)
		u.state, u.err = Failed, err.Error()
		return
	}
	u.state, u.staged = Ready, staged
	if u.onReady != nil {
		go u.onReady(rel.Version)
	}
}

// Status is where the updater stands.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	s := Status{State: u.state, Current: u.current, Stuck: u.stuck, Error: u.err, Done: u.done, Total: u.total}
	if u.latest != nil {
		s.Latest, s.Notes, s.URL = u.latest.Version, u.latest.Notes, u.latest.URL
	}
	return s
}

// install puts the staged version in; ask lets it put up the password
// prompt. A failure keeps the stage, for another try.
func (u *Updater) install(ask bool) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.staged == "" {
		return errors.New("no update is ready")
	}
	err := Install(u.staged, u.bundle)
	if ask && NeedsAdmin(err) {
		err = InstallAsAdmin(u.staged, u.bundle)
	}
	if err != nil {
		if !errors.Is(err, ErrCanceled) {
			log.Println("pulsewise: update:", err)
			u.err = err.Error()
		}
		return err
	}
	u.staged, u.state = "", Current
	return nil
}

// Restart puts the staged version in and has it opened once this process
// has exited; the caller quits next. When it can't, it says why and the
// app runs on.
func (u *Updater) Restart() error {
	if err := u.install(true); err != nil {
		return err
	}
	return Relaunch(u.bundle)
}

// Quitting puts a staged version in on the way out, without the password
// prompt, so the next launch is the new one.
func (u *Updater) Quitting() {
	u.mu.Lock()
	ready := u.staged != ""
	u.mu.Unlock()
	if ready {
		_ = u.install(false)
	}
}

// OpenPage opens the latest release's page, for an app that can't replace
// itself.
func (u *Updater) OpenPage() error {
	u.mu.Lock()
	url := ""
	if u.latest != nil {
		url = u.latest.URL
	}
	u.mu.Unlock()
	// the feed's word: open is only given a web page
	if !strings.HasPrefix(url, "https://") {
		return errors.New("no release page")
	}
	return exec.Command("open", url).Start()
}
