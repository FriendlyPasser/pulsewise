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

const (
	loginTooltipDefault          = "登录时自动启动 Pulsewise"
	loginTooltipRequiresApproval = "已在系统设置中禁用，点击前往开启"
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
}

func (p wailsLoginPrompt) confirmOpenSettings() {
	dialog := p.app.Dialog.Question().
		SetTitle("需要在系统设置中允许").
		SetMessage("Pulsewise 的登录启动项已在系统设置中被禁用。请前往“系统设置 > 通用 > 登录项”重新开启。")
	openBtn := dialog.AddButton("打开系统设置").OnClick(func() {
		if err := p.driver.openSystemSettings(); err != nil {
			log.Println("pulsewise: open login items settings:", err)
			p.showError("无法打开系统设置", "请手动前往“系统设置 > 通用 > 登录项”开启 Pulsewise。")
		}
	})
	cancelBtn := dialog.AddButton("取消")
	dialog.SetDefaultButton(openBtn).SetCancelButton(cancelBtn)
	dialog.Show()
}

func (p wailsLoginPrompt) showError(title, message string) {
	p.app.Dialog.Error().SetTitle(title).SetMessage(message).Show()
}

func applyLoginItemPresentation(item *application.MenuItem, status launchStatus) {
	if item == nil {
		return
	}
	tooltip := loginTooltipDefault
	if status == launchRequiresApproval {
		tooltip = loginTooltipRequiresApproval
	}
	item.SetChecked(status.enabled()).SetTooltip(tooltip)
}

func syncLoginItem(item *application.MenuItem, driver loginItemDriver) launchStatus {
	if item == nil {
		return launchUnavailable
	}
	status, err := driver.status()
	if err != nil {
		log.Println("pulsewise: login item status:", err)
		status = launchUnavailable
	}
	applyLoginItemPresentation(item, status)
	return status
}

func toggleLoginItem(item *application.MenuItem, driver loginItemDriver, prompt loginPrompt) {
	current, err := driver.status()
	if err != nil {
		log.Println("pulsewise: login item status:", err)
		applyLoginItemPresentation(item, launchUnavailable)
		prompt.showError("设置登录启动失败", "无法读取当前登录启动状态，请稍后重试。")
		return
	}
	if current == launchRequiresApproval {
		applyLoginItemPresentation(item, current)
		prompt.confirmOpenSettings()
		return
	}
	if err := driver.setEnabled(!current.enabled()); err != nil {
		if errors.Is(err, errLaunchRequiresApproval) {
			applyLoginItemPresentation(item, launchRequiresApproval)
			prompt.confirmOpenSettings()
			return
		}
		log.Println("pulsewise: set launch at login:", err)
		applyLoginItemPresentation(item, current)
		prompt.showError("设置登录启动失败", "无法更改登录时打开设置，请稍后重试。")
		return
	}
	if syncLoginItem(item, driver) == launchRequiresApproval {
		prompt.confirmOpenSettings()
	}
}
