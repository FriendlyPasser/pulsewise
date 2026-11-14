package gui

import "github.com/FriendlyPasser/pulsewise/internal/panel"

type guiMessages struct {
	adminUpdatePrompt string

	menuDisplay       string
	savePrefsErrTitle string
	savePrefsErrMsg   string
	keepAtLeastOne    string
	restoreDefaults   string
	openAtLogin       string
	checkForUpdates   string
	quitApp           string

	traySeparator   string
	trayUnavailable string
	metricMemory    string
	metricDisk      string

	loginTooltipDefault          string
	loginTooltipRequiresApproval string
	loginApprovalTitle           string
	loginApprovalMsg             string
	loginOpenSettingsBtn         string
	loginCancelBtn               string
	loginOpenSettingsErrTitle    string
	loginOpenSettingsErrMsg      string
	loginSetErrTitle             string
	loginReadErrMsg              string
	loginChangeErrMsg            string
}

var guiMessagesZhCN = guiMessages{
	adminUpdatePrompt: "Pulsewise 正在更新。",

	menuDisplay:       "菜单栏显示",
	savePrefsErrTitle: "无法保存显示设置",
	savePrefsErrMsg:   "显示设置未保存，请稍后重试。",
	keepAtLeastOne:    "至少保留一项",
	restoreDefaults:   "恢复默认",
	openAtLogin:       "登录时打开",
	checkForUpdates:   "检查更新",
	quitApp:           "退出 Pulsewise",

	traySeparator:   "，",
	trayUnavailable: "不可用",
	metricMemory:    "内存",
	metricDisk:      "硬盘",

	loginTooltipDefault:          "登录时自动启动 Pulsewise",
	loginTooltipRequiresApproval: "已在系统设置中禁用，点击前往开启",
	loginApprovalTitle:           "需要在系统设置中允许",
	loginApprovalMsg:             "Pulsewise 的登录启动项已在系统设置中被禁用。请前往“系统设置 > 通用 > 登录项”重新开启。",
	loginOpenSettingsBtn:         "打开系统设置",
	loginCancelBtn:               "取消",
	loginOpenSettingsErrTitle:    "无法打开系统设置",
	loginOpenSettingsErrMsg:      "请手动前往“系统设置 > 通用 > 登录项”开启 Pulsewise。",
	loginSetErrTitle:             "设置登录启动失败",
	loginReadErrMsg:              "无法读取当前登录启动状态，请稍后重试。",
	loginChangeErrMsg:            "无法更改登录时打开设置，请稍后重试。",
}

var guiMessagesEn = guiMessages{
	adminUpdatePrompt: "Pulsewise is updating.",

	menuDisplay:       "Menu Bar Display",
	savePrefsErrTitle: "Unable to Save Display Settings",
	savePrefsErrMsg:   "Display settings were not saved. Please try again later.",
	keepAtLeastOne:    "Keep at least one",
	restoreDefaults:   "Restore Defaults",
	openAtLogin:       "Open at Login",
	checkForUpdates:   "Check for Updates",
	quitApp:           "Quit Pulsewise",

	traySeparator:   ", ",
	trayUnavailable: "unavailable",
	metricMemory:    "Memory",
	metricDisk:      "Disk",

	loginTooltipDefault:          "Launch Pulsewise automatically at login",
	loginTooltipRequiresApproval: "Disabled in System Settings. Click to enable.",
	loginApprovalTitle:           "Allow in System Settings",
	loginApprovalMsg:             "Pulsewise is disabled under Login Items in System Settings. Open System Settings > General > Login Items to enable it.",
	loginOpenSettingsBtn:         "Open System Settings",
	loginCancelBtn:               "Cancel",
	loginOpenSettingsErrTitle:    "Unable to Open System Settings",
	loginOpenSettingsErrMsg:      "Please open System Settings > General > Login Items to enable Pulsewise manually.",
	loginSetErrTitle:             "Open at Login Error",
	loginReadErrMsg:              "Unable to check the current Open at Login status. Please try again later.",
	loginChangeErrMsg:            "Unable to change the Open at Login setting. Please try again later.",
}

func messagesForLocale(loc panel.Locale) guiMessages {
	if loc.IsZh() {
		return guiMessagesZhCN
	}
	return guiMessagesEn
}
