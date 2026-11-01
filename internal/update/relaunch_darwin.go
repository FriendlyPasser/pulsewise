package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

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
