//go:build !darwin || !cgo

package gui

func newLoginItemDriver() loginItemDriver {
	return unsupportedLoginItemDriver{}
}

type unsupportedLoginItemDriver struct{}

func (unsupportedLoginItemDriver) available() bool { return false }

func (unsupportedLoginItemDriver) status() (launchStatus, error) {
	return launchUnavailable, errLaunchUnsupported
}

func (unsupportedLoginItemDriver) setEnabled(bool) error {
	return errLaunchUnsupported
}

func (unsupportedLoginItemDriver) openSystemSettings() error {
	return errLaunchUnsupported
}
