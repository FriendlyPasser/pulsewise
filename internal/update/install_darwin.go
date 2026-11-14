package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

const defaultAdminPrompt = "Pulsewise is updating."

// InstallAsAdmin is Install with the administrator's password, asked for
// by the system.
func InstallAsAdmin(staged, bundle string) error {
	return installAsAdmin(staged, bundle, defaultAdminPrompt)
}

func installAsAdmin(staged, bundle, prompt string) error {
	if prompt == "" {
		prompt = defaultAdminPrompt
	}
	old := filepath.Join(filepath.Dir(staged), "old.app")
	err := asAdmin(swapScript(staged, bundle, old), prompt)
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
func asAdmin(script, prompt string) error {
	out, err := exec.Command("osascript", "-e",
		`do shell script "`+appleString(script)+`" with prompt "`+appleString(prompt)+`" with administrator privileges`).CombinedOutput()
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
