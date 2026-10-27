package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.1.2", "0.1.1", true},
		{"0.2", "0.1.9", true},
		{"v1.0.0", "0.9.9", true},
		{"0.1.10", "0.1.9", true},
		{"0.1.1", "0.1.1", false},
		{"0.1", "0.1.0", false},
		{"0.1.0", "0.1.1", false},
		{"dev", "0.1.0", false},
		{"0.1.1-3-gabc", "0.1.0", false},
		{"1.2.3.4", "0.1.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestBundleOf(t *testing.T) {
	if got := bundleOf("/Applications/Pulsewise.app/Contents/MacOS/pulsewise"); got != "/Applications/Pulsewise.app" {
		t.Errorf("in a bundle: %q", got)
	}
	for _, exe := range []string{"/usr/local/bin/pulsewise", "/tmp/go-build/exe/pulsewise", "/x/MacOS/pulsewise"} {
		if got := bundleOf(exe); got != "" {
			t.Errorf("bundleOf(%q) = %q", exe, got)
		}
	}
}

func TestVersionAndExecutable(t *testing.T) {
	app := fakeApp(t, t.TempDir(), "0.3.1")
	if v := Version(app); v != "0.3.1" {
		t.Errorf("Version = %q", v)
	}
	if e := executableOf(app); e != "pulsewise" {
		t.Errorf("executableOf = %q", e)
	}
	if v := Version(t.TempDir()); v != "" {
		t.Errorf("no Info.plist: %q", v)
	}
}

func TestStuck(t *testing.T) {
	if s := Stuck("/private/var/folders/x/AppTranslocation/ABC/d/Pulsewise.app"); s != "translocated" {
		t.Errorf("translocated: %q", s)
	}
	if s := Stuck(t.TempDir()); s != "" {
		t.Errorf("a writable folder: %q", s)
	}
}

func TestTeamOf(t *testing.T) {
	out := "Executable=/x\nIdentifier=io.github.friendlypasser.pulsewise\nTeamIdentifier=AB12CD34EF\n"
	if got := teamOf(out); got != "AB12CD34EF" {
		t.Errorf("signed: %q", got)
	}
	if got := teamOf("Signature=adhoc\nTeamIdentifier=not set\n"); got != "" {
		t.Errorf("ad hoc: %q", got)
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Write([]byte(`{"version":"0.2.0","notes":"n","url":"https://x/r","zip":{"url":"https://x/z.zip","size":5,"sha256":"ab"}}`))
		case "/bad.json":
			w.Write([]byte(`{"version":"next"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("PULSEWISE_UPDATE_FEED", srv.URL+"/latest.json")
	r, err := Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "0.2.0" || r.Zip.SHA256 != "ab" || r.Zip.Size != 5 {
		t.Errorf("%+v", r)
	}
	for _, path := range []string{"/bad.json", "/missing.json"} {
		t.Setenv("PULSEWISE_UPDATE_FEED", srv.URL+path)
		if _, err := Latest(context.Background()); err == nil {
			t.Errorf("%s: no error", path)
		}
	}
}

func TestDownloadChecksHash(t *testing.T) {
	body := []byte("the new app")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }))
	defer srv.Close()
	sum := sha256.Sum256(body)
	good := hex.EncodeToString(sum[:])
	path := filepath.Join(t.TempDir(), "update.zip")

	var done, total int64
	err := download(context.Background(), Asset{URL: srv.URL + "/z.zip", SHA256: strings.ToUpper(good)}, path,
		func(d, n int64) { done, total = d, n })
	if err != nil {
		t.Fatal(err)
	}
	if done != int64(len(body)) || total != int64(len(body)) {
		t.Errorf("progress %d of %d", done, total)
	}

	err = fetch(context.Background(), Asset{URL: srv.URL + "/z.zip", SHA256: strings.Repeat("0", 64)}, path, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("a wrong hash: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a download that failed its hash was kept")
	}
	if err := download(context.Background(), Asset{URL: srv.URL}, path, nil); err == nil {
		t.Error("no checksum: no error")
	}
}

func TestInstallSwapsBundle(t *testing.T) {
	dir := t.TempDir()
	bundle := fakeApp(t, dir, "0.1.0")
	staged := fakeApp(t, filepath.Join(dir, ".pulsewise-update", "app"), "0.2.0")
	if err := Install(staged, bundle); err != nil {
		t.Fatal(err)
	}
	if v := Version(bundle); v != "0.2.0" {
		t.Errorf("after install: %q", v)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pulsewise-update")); !os.IsNotExist(err) {
		t.Error("the stage was left behind")
	}
}

func TestInstallPutsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	bundle := fakeApp(t, dir, "0.1.0")
	missing := filepath.Join(dir, ".pulsewise-update", "app", "Pulsewise.app")
	os.MkdirAll(filepath.Dir(missing), 0o755)
	if err := Install(missing, bundle); err == nil {
		t.Fatal("no error")
	}
	if v := Version(bundle); v != "0.1.0" {
		t.Errorf("the old app wasn't put back: %q", v)
	}
}

func TestSameSignerRefusesNoTeam(t *testing.T) {
	if _, err := exec.LookPath("codesign"); err != nil {
		t.Skip("no codesign")
	}
	dir := t.TempDir()
	a, b := fakeApp(t, filepath.Join(dir, "a"), "0.1.0"), fakeApp(t, filepath.Join(dir, "b"), "0.2.0")
	for _, app := range []string{a, b} {
		if out, err := exec.Command("codesign", "--force", "--sign", "-", app).CombinedOutput(); err != nil {
			t.Skipf("ad hoc signing: %v: %s", err, out)
		}
	}
	// two ad hoc signatures match in having no team, which proves nothing
	if err := sameSigner(context.Background(), a, b); err == nil {
		t.Error("an ad hoc app accepted an ad hoc download")
	}
}

func TestRelaunchScript(t *testing.T) {
	s := relaunchScript(42, "/Applications/Pulse wise.app", []string{"0.5", "1"})
	for _, want := range []string{
		"while kill -0 42",
		"for w in 0.5 1; do open -n '/Applications/Pulse wise.app' && exit 0; sleep $w; done",
		"exec '/Applications/Pulse wise.app/Contents/MacOS/pulsewise'",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in\n%s", want, s)
		}
	}
}

func TestRelaunchScriptRuns(t *testing.T) {
	// a stand-in bundle whose open fails (no such app) and whose executable
	// writes a file: the script falls back to it once the pid is gone
	dir := t.TempDir()
	app := filepath.Join(dir, "Nothing.app")
	mark := filepath.Join(dir, "ran")
	exe := filepath.Join(app, "Contents", "MacOS", "pulsewise")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("#!/bin/sh\ntouch "+shellQuote(mark)+"\n"), 0o755)

	gone := exec.Command("true")
	gone.Run()
	cmd := exec.Command("/bin/sh", "-c", relaunchScript(gone.Process.Pid, app, []string{"0"}))
	bin := filepath.Join(dir, "bin") // an open that always fails
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(mark); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("the bundle's executable was never run")
}

func TestUpdaterOffOutsideBundle(t *testing.T) {
	u := New() // the test binary is in no bundle
	if s := u.Status(); s.State != Off {
		t.Errorf("state %q", s.State)
	}
	u.Check()
	u.Start()
	if s := u.Status(); s.State != Off {
		t.Errorf("after Check: %q", s.State)
	}
}

func TestOpenPageOnlyWeb(t *testing.T) {
	u := &Updater{latest: &Release{URL: "file:///Applications/Calculator.app"}}
	if err := u.OpenPage(); err == nil {
		t.Error("opened a file URL")
	}
}

func fakeApp(t *testing.T, dir, version string) string {
	t.Helper()
	app := filepath.Join(dir, "Pulsewise.app")
	macos := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
	<key>CFBundleExecutable</key>
	<string>pulsewise</string>
	<key>CFBundleShortVersionString</key>
	<string>` + version + `</string>
</dict></plist>
`
	os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644)
	os.WriteFile(filepath.Join(macos, "pulsewise"), []byte("#!/bin/sh\n"), 0o755)
	return app
}
