package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

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
