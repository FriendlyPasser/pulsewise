// Package update keeps Pulsewise current, after yetone/magpie's: a feed
// (latest.json, written by scripts/release.sh next to each release's zip)
// names the newest version and its zip with the zip's SHA-256. A newer one
// is downloaded, checked against that hash, unpacked beside the installed
// bundle, and accepted only if it is signed by the same team as the app
// already installed. Restarting swaps it in (two renames) and opens it
// once this process has exited; quitting swaps it in too.
//
// Only a release updates itself: an app run from its bundle, with a
// version in its Info.plist and a team in its signature. `go run` and the
// -web mode have neither and never ask the feed.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// feedURL is where the newest release is described. release.sh can bake
// another in with -ldflags "-X …/update.feedURL=…"; PULSEWISE_UPDATE_FEED
// points it elsewhere at run time, for trying an update against a local
// server.
var feedURL = "https://github.com/FriendlyPasser/pulsewise/releases/latest/download/latest.json"

// Feed is the feed's URL.
func Feed() string {
	if f := os.Getenv("PULSEWISE_UPDATE_FEED"); f != "" {
		return f
	}
	return feedURL
}

// Release is one published version, as latest.json describes it.
type Release struct {
	Version string `json:"version"` // "0.2.0", no v
	Notes   string `json:"notes"`
	URL     string `json:"url"` // the release page
	Zip     Asset  `json:"zip"`
}

// Asset is a downloadable file of a release.
type Asset struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

var client = &http.Client{Timeout: 10 * time.Minute}

// Latest asks the feed for the newest release.
func Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", Feed(), nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update feed: %s", res.Status)
	}
	var r Release
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&r); err != nil {
		return nil, fmt.Errorf("update feed: %w", err)
	}
	if parse(r.Version) == nil {
		return nil, fmt.Errorf("update feed: bad version %q", r.Version)
	}
	return &r, nil
}

// Newer reports whether version a comes after b. Versions are release.sh's:
// one to three numbers, a leading v allowed.
func Newer(a, b string) bool {
	x, y := parse(a), parse(b)
	if x == nil || y == nil {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

func parse(v string) []int {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nil
	}
	n := make([]int, 3)
	for i, p := range parts {
		d, err := strconv.Atoi(p)
		if err != nil || d < 0 {
			return nil
		}
		n[i] = d
	}
	return n
}

// Bundle is the .app the running binary lives in, or "" when it is not in
// one.
func Bundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return ""
	}
	return bundleOf(exe)
}

// bundleOf is the .app of …/Name.app/Contents/MacOS/exe, or "".
func bundleOf(exe string) string {
	macos := filepath.Dir(exe)
	app := filepath.Dir(filepath.Dir(macos))
	if filepath.Base(macos) != "MacOS" || filepath.Ext(app) != ".app" {
		return ""
	}
	return app
}

var shortVersion = regexp.MustCompile(`<key>CFBundleShortVersionString</key>\s*<string>([^<]+)</string>`)

// Version is the version in bundle's Info.plist, or "".
func Version(bundle string) string {
	b, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		return ""
	}
	if m := shortVersion.FindSubmatch(b); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

// Stuck says why the app at bundle can't be replaced where it is, or ""
// when it can: run from its disk image, or moved aside by macOS (App
// Translocation) because it was opened before being moved out of
// Downloads.
func Stuck(bundle string) string {
	switch {
	case strings.Contains(bundle, "/AppTranslocation/"):
		return "translocated"
	case readOnly(bundle):
		return "read-only"
	}
	return ""
}

// Stage downloads rel's zip, checks it, and unpacks it beside bundle,
// ready for Install. It returns the unpacked app. progress, when set, is
// told the bytes so far and the total (0 while unknown).
func Stage(ctx context.Context, rel *Release, bundle string, progress func(done, total int64)) (string, error) {
	dir := stageDir(bundle)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	zip := filepath.Join(dir, "update.zip")
	if err := download(ctx, rel.Zip, zip, progress); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	out := filepath.Join(dir, "app")
	if b, err := exec.CommandContext(ctx, "ditto", "-x", "-k", zip, out).CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("unzip: %v: %s", err, b)
	}
	os.Remove(zip)
	app := filepath.Join(out, filepath.Base(bundle))
	if err := sameSigner(ctx, bundle, app); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return app, nil
}

// stageDir is where an update is unpacked: beside the bundle, so putting
// it in is a rename on one volume. Where that folder isn't Pulsewise's to
// write, the user's cache, to be moved in with the administrator's
// password.
func stageDir(bundle string) string {
	parent := filepath.Dir(bundle)
	if writable(parent) {
		return filepath.Join(parent, ".pulsewise-update")
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cache, "Pulsewise", "update")
	}
	return filepath.Join(os.TempDir(), "pulsewise-update")
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".pulsewise-update-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// sameSigner accepts app only if its signature is intact and made by the
// team that signed bundle. A bundle with no team (ad hoc, or unsigned)
// accepts nothing: anyone can make an ad hoc signature.
func sameSigner(ctx context.Context, bundle, app string) error {
	if b, err := exec.CommandContext(ctx, "codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("the downloaded app's signature is broken: %s", strings.TrimSpace(string(b)))
	}
	want, got := Team(ctx, bundle), Team(ctx, app)
	if want == "" {
		return errors.New("the installed app has no signing team to check the download against")
	}
	if want != got {
		return fmt.Errorf("the downloaded app is signed by %q, not %q", got, want)
	}
	return nil
}

// Team is the TeamIdentifier of app's signature, or "".
func Team(ctx context.Context, app string) string {
	b, _ := exec.CommandContext(ctx, "codesign", "-dv", app).CombinedOutput()
	return teamOf(string(b))
}

func teamOf(codesignOut string) string {
	for _, l := range strings.Split(codesignOut, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "TeamIdentifier="); ok && v != "not set" {
			return v
		}
	}
	return ""
}

// download fetches a to path and checks its hash. A connection that drops
// part way gets two more tries.
func download(ctx context.Context, a Asset, path string, progress func(done, total int64)) error {
	if a.URL == "" {
		return errors.New("the release names no zip")
	}
	if a.SHA256 == "" {
		return errors.New("the release lists no checksum for its zip")
	}
	var err error
	for try := range 3 {
		if try > 0 {
			select {
			case <-ctx.Done():
				return err
			case <-time.After(time.Duration(try) * 2 * time.Second):
			}
		}
		if err = fetch(ctx, a, path, progress); err == nil || ctx.Err() != nil {
			break
		}
	}
	return err
}

func fetch(ctx context.Context, a Asset, path string, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", filepath.Base(a.URL), res.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	var body io.Reader = res.Body
	if progress != nil {
		total := res.ContentLength
		if total <= 0 {
			total = a.Size
		}
		body = &counter{r: res.Body, total: max(total, 0), report: progress}
		progress(0, max(total, 0))
	}
	_, err = io.Copy(io.MultiWriter(f, h), body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), a.SHA256) {
		err = fmt.Errorf("%s does not match its checksum", filepath.Base(a.URL))
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// counter reports bytes as they are read.
type counter struct {
	r      io.Reader
	done   int64
	total  int64
	report func(done, total int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	c.report(c.done, c.total)
	return n, err
}

// Install swaps the staged app in for bundle: the bundle moved aside, the
// new one renamed into its place, the old one removed. The running copy
// keeps going until it exits. Where the folder isn't Pulsewise's to
// change the error is a permission one (NeedsAdmin), and InstallAsAdmin
// can do it instead.
func Install(staged, bundle string) error {
	old := filepath.Join(filepath.Dir(staged), "old.app")
	os.RemoveAll(old)
	if err := os.Rename(bundle, old); err != nil {
		return err
	}
	if err := os.Rename(staged, bundle); err != nil {
		os.Rename(old, bundle) // put it back
		return err
	}
	os.RemoveAll(filepath.Dir(filepath.Dir(staged))) // the stage dir, old.app with it
	return nil
}

// NeedsAdmin is whether err is a swap the user may not make: a folder
// that is root's, or a stage on another volume than the bundle.
func NeedsAdmin(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, errCrossDevice)
}

// ErrCanceled is the password prompt dismissed.
var ErrCanceled = errors.New("canceled")

// InstallAsAdmin is Install with the administrator's password, asked for
// by the system.
func InstallAsAdmin(staged, bundle string) error {
	old := filepath.Join(filepath.Dir(staged), "old.app")
	err := asAdmin(swapScript(staged, bundle, old))
	if err == nil {
		os.RemoveAll(filepath.Dir(filepath.Dir(staged)))
	}
	return err
}

func swapScript(staged, bundle, old string) string {
	s, b, o := shellQuote(staged), shellQuote(bundle), shellQuote(old)
	return "rm -rf " + o + " && mv " + b + " " + o + " && if mv " + s + " " + b + "; then rm -rf " + o + "; else mv " + o + " " + b + "; exit 1; fi"
}

// asAdmin runs a shell script as root, after the system's password prompt.
func asAdmin(script string) error {
	out, err := exec.Command("osascript", "-e",
		`do shell script "`+appleString(script)+`" with prompt "Pulsewise 正在更新。" with administrator privileges`).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "(-128)") {
			return ErrCanceled
		}
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New(msg)
		}
	}
	return err
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func appleString(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

// openWaits are the pauses, in seconds, between tries of the relaunch's
// open. Just after a process exits LaunchServices can still hold it as
// running, and open fails (-600); with the screen locked it can go on
// refusing, so after the last try the bundle's executable is run straight.
var openWaits = []string{"0.5", "1", "1", "2", "2", "3", "4"}

// Relaunch opens bundle again once this process has exited. Call it, then
// quit.
func Relaunch(bundle string) error {
	cmd := exec.Command("/bin/sh", "-c", relaunchScript(os.Getpid(), bundle, openWaits))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	// a session of its own: launchd ends the process group of a job whose
	// process exits, which would take this shell with it
	detach(cmd)
	return cmd.Start()
}

// relaunchScript waits for pid to exit, then opens bundle as a new
// instance (-n: without it open may send a reopen event to the process
// that just went), tried again while it fails, and at last runs the
// bundle's executable by its path.
func relaunchScript(pid int, bundle string, waits []string) string {
	exe := filepath.Join(bundle, "Contents", "MacOS", executableOf(bundle))
	open := "open -n " + shellQuote(bundle)
	var b strings.Builder
	fmt.Fprintf(&b, "while kill -0 %d 2>/dev/null; do sleep 0.2; done; ", pid)
	fmt.Fprintf(&b, "for w in %s; do %s && exit 0; sleep $w; done; %s && exit 0; ", strings.Join(waits, " "), open, open)
	fmt.Fprintf(&b, "exec %s </dev/null >/dev/null 2>&1", shellQuote(exe))
	return b.String()
}

var bundleExecutable = regexp.MustCompile(`<key>CFBundleExecutable</key>\s*<string>([^<]+)</string>`)

// executableOf is the program bundle's Info.plist names, else pulsewise.
func executableOf(bundle string) string {
	if b, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist")); err == nil {
		if m := bundleExecutable.FindSubmatch(b); m != nil {
			if name := strings.TrimSpace(string(m[1])); name != "" && !strings.ContainsRune(name, '/') {
				return name
			}
		}
	}
	return "pulsewise"
}
