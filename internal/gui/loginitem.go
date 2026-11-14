package gui

import (
	"errors"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type launchStatus uint8

const (
	launchDisabled launchStatus = iota
	launchEnabled
	launchRequiresApproval
	launchUnavailable
)

func (s launchStatus) enabled() bool {
	return s == launchEnabled
}

var (
	errLaunchRequiresApproval = errors.New("launch at login requires approval in System Settings")
	errLaunchUnsupported      = errors.New("launch at login is not supported on this platform")
)

type loginItemDriver interface {
	available() bool
	status() (launchStatus, error)
	setEnabled(enabled bool) error
	openSystemSettings() error
}

type loginPrompt interface {
	confirmOpenSettings()
	showError(title, message string)
}

type wailsLoginPrompt struct {
	app    *application.App
	driver loginItemDriver
	msg    guiMessages
}

func (p wailsLoginPrompt) confirmOpenSettings() {
	dialog := p.app.Dialog.Question().
		SetTitle(p.msg.loginApprovalTitle).
		SetMessage(p.msg.loginApprovalMsg)
	openBtn := dialog.AddButton(p.msg.loginOpenSettingsBtn).OnClick(func() {
		if err := p.driver.openSystemSettings(); err != nil {
			log.Println("pulsewise: open login items settings:", err)
			p.showError(p.msg.loginOpenSettingsErrTitle, p.msg.loginOpenSettingsErrMsg)
		}
	})
	cancelBtn := dialog.AddButton(p.msg.loginCancelBtn)
	dialog.SetDefaultButton(openBtn).SetCancelButton(cancelBtn)
	dialog.Show()
}

func (p wailsLoginPrompt) showError(title, message string) {
	p.app.Dialog.Error().SetTitle(title).SetMessage(message).Show()
}

func applyLoginItemPresentation(item *application.MenuItem, status launchStatus, msg guiMessages) {
	if item == nil {
		return
	}
	tooltip := msg.loginTooltipDefault
	if status == launchRequiresApproval {
		tooltip = msg.loginTooltipRequiresApproval
	}
	item.SetChecked(status.enabled()).SetTooltip(tooltip)
}

func syncLoginItem(item *application.MenuItem, driver loginItemDriver, msg guiMessages) launchStatus {
	if item == nil {
		return launchUnavailable
	}
	status, err := driver.status()
	if err != nil {
		log.Println("pulsewise: login item status:", err)
		status = launchUnavailable
	}
	applyLoginItemPresentation(item, status, msg)
	return status
}

func toggleLoginItem(item *application.MenuItem, driver loginItemDriver, prompt loginPrompt, msg guiMessages) {
	current, err := driver.status()
	if err != nil {
		log.Println("pulsewise: login item status:", err)
		applyLoginItemPresentation(item, launchUnavailable, msg)
		prompt.showError(msg.loginSetErrTitle, msg.loginReadErrMsg)
		return
	}
	if current == launchRequiresApproval {
		applyLoginItemPresentation(item, current, msg)
		prompt.confirmOpenSettings()
		return
	}
	if err := driver.setEnabled(!current.enabled()); err != nil {
		if errors.Is(err, errLaunchRequiresApproval) {
			applyLoginItemPresentation(item, launchRequiresApproval, msg)
			prompt.confirmOpenSettings()
			return
		}
		log.Println("pulsewise: set launch at login:", err)
		applyLoginItemPresentation(item, current, msg)
		prompt.showError(msg.loginSetErrTitle, msg.loginChangeErrMsg)
		return
	}
	if syncLoginItem(item, driver, msg) == launchRequiresApproval {
		prompt.confirmOpenSettings()
	}
}
