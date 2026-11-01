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

// Every is the interval between automatic update checks.
const Every = time.Minute

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
	ctx     context.Context
	bundle  string
	current string
	stuck   string

	mu     sync.Mutex
	state  State
	latest *Release
	err    string
	staged string
	done   int64
	total  int64
}

// New ties checks and downloads to the application's context. The updater is
// Off outside a bundle, or when the app has no version or signing team.
func New(ctx context.Context) *Updater {
	u := &Updater{ctx: ctx, state: Off}
	b := Bundle()
	if b == "" {
		return u
	}
	v := Version(b)
	if parse(v) == nil || Team(ctx, b) == "" {
		return u
	}
	u.bundle, u.current, u.stuck, u.state = b, v, Stuck(b), Idle
	return u
}

// Run checks after five seconds and then every Every, until the application's
// context is canceled. Missed ticks are dropped instead of replayed after sleep.
func (u *Updater) Run() {
	if u.Status().State == Off {
		return
	}
	delay := time.NewTimer(5 * time.Second)
	defer delay.Stop()
	select {
	case <-u.ctx.Done():
		return
	case <-delay.C:
	}
	tick := time.NewTicker(Every)
	defer tick.Stop()
	for {
		u.Check()
		select {
		case <-u.ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Check asks the feed now, unless a check or a download is under way, and
// downloads what is newer. It returns at once.
func (u *Updater) Check() {
	u.mu.Lock()
	if u.state == Off || u.state == Checking || u.state == Downloading || u.ctx.Err() != nil {
		u.mu.Unlock()
		return
	}
	u.state, u.err = Checking, ""
	u.mu.Unlock()
	go u.run()
}

func (u *Updater) run() {
	ctx, cancel := context.WithTimeout(u.ctx, 10*time.Minute)
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
