// Package gui hosts the native menu bar, panel window, and application lifecycle.
package gui

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/FriendlyPasser/pulsewise/internal/stats"
	"github.com/FriendlyPasser/pulsewise/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// panelWidth is the design's 340px panel plus the room its shadow takes.
const (
	panelWidth  = 340 + 2*panelShadow
	panelShadow = 16
)

type host struct {
	app               *application.App
	panel             *application.WebviewWindow
	chooseFolderTitle string
	chooseFolderBtn   string
	diskScanner       *stats.DiskScanner
	revealFn          func(string) error
}

func (h *host) FitPanel(height int) {
	application.InvokeSync(func() { h.panel.SetSize(panelWidth, height) })
}

func (h *host) openDetailWindow(path string) {
	application.InvokeAsync(func() {
		if h.panel != nil && h.panel.IsVisible() {
			h.panel.Hide()
		}
		if w, ok := h.app.Window.GetByName(detailWindowName); ok && w != nil {
			w.ExecJS("if (location.pathname !== " + strconv.Quote(path) + ") location.assign(" + strconv.Quote(path) + ")")
			w.Restore()
			w.Show()
			w.Focus()
			return
		}
		win := h.app.Window.NewWithOptions(detailWindowOptions(path))
		win.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(_ *application.WindowEvent) {
			application.InvokeAsync(func() {
				win.Show()
				win.Focus()
			})
		})
		win.OnWindowEvent(events.Mac.WindowWillClose, func(_ *application.WindowEvent) {
			if h.diskScanner != nil {
				h.diskScanner.Cancel()
			}
		})
		win.Show()
		win.Focus()
	})
}

func (h *host) OpenMemory() {
	h.openDetailWindow("/memory.html")
}

func (h *host) OpenDisk() {
	h.openDetailWindow("/disk.html")
}

func (h *host) ChooseDiskDirectory(initial string) (string, bool, error) {
	dlg := h.app.Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(false).
		SetTitle(h.chooseFolderTitle).
		SetMessage(h.chooseFolderTitle).
		SetButtonText(h.chooseFolderBtn)
	if initial != "" {
		if dir := resolveExistingDir(initial); dir != "" {
			dlg.SetDirectory(dir)
		}
	}
	if w, ok := h.app.Window.GetByName(detailWindowName); ok && w != nil {
		dlg.AttachToWindow(w)
	}
	path, err := dlg.PromptForSingleSelection()
	return path, path == "", err
}

func resolveExistingDir(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		return ""
	}
	clean := filepath.Clean(p)
	if info, err := os.Stat(clean); err == nil && info.IsDir() {
		return clean
	}
	return ""
}

func (h *host) RevealInFinder(path string) error {
	reveal := h.revealFn
	if reveal == nil {
		reveal = revealInFinder
	}
	return reveal(path)
}

func revealInFinder(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return errors.New("invalid path")
	}
	return exec.Command("/usr/bin/open", "-R", path).Run()
}

func (h *host) Quit() { h.app.Quit() }

// Run starts the app and blocks until it quits.
//
// It sets no app icon: a menu bar extra shows it nowhere, and Wails
// decodes the 1024px PNG into a bitmap that kept 32 MB, half the app's
// memory. The Finder's icon is the .app bundle's job (build/icon).
func Run() error {
	loc := panel.SystemLocale()
	msg := messagesForLocale(loc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampler := stats.NewSampler(time.Second)
	diskScanner := stats.NewDiskScanner(ctx)
	updater := update.New(ctx, update.WithAdminPrompt(msg.adminUpdatePrompt))
	selected := defaultTrayMetrics
	preferencesPath, err := trayPreferencesPath()
	if err == nil {
		selected, err = loadTrayMetrics(preferencesPath)
	}
	if err != nil {
		log.Println("pulsewise: tray preferences:", err)
	}

	h := &host{
		chooseFolderTitle: msg.chooseFolderTitle,
		chooseFolderBtn:   msg.chooseFolderBtn,
		diskScanner:       diskScanner,
	}
	h.app = application.New(application.Options{
		Name:        "Pulsewise",
		Description: "CPU, GPU, memory and disk capacity in the menu bar",
		Assets:      application.AssetOptions{Handler: panel.Handler(h, sampler, diskScanner, updater, loc)},
		// a version downloaded and not restarted into goes in on the way
		// out, so the next launch is the new one
		OnShutdown: func() {
			cancel()
			updater.Quitting()
		},
		// a menu bar extra: no Dock icon, no Command-Tab
		Mac:          application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		ErrorHandler: func(err error) { log.Println("pulsewise:", err) },
	})

	h.panel = h.app.Window.NewWithOptions(panelOptions())

	tray := h.app.SystemTray.New()
	// After startup, selection and rendering are owned by the native UI thread.
	refreshTray := func(s stats.Snapshot) {
		icon, tip := trayPresentation(s, selected, loc)
		tray.SetTemplateIcon(icon)
		tray.SetTooltip(tip)
	}
	samples := sampler.Subscribe(ctx)
	snap := <-samples
	refreshTray(snap)
	tray.AttachWindow(h.panel).WindowOffset(6)

	menu := h.app.NewMenu()
	display := menu.AddSubmenu(msg.menuDisplay)
	items := make(map[trayMetrics]*application.MenuItem, len(trayOptions))
	applySelection := func(next trayMetrics) {
		if err := saveTrayMetrics(preferencesPath, next); err != nil {
			log.Println("pulsewise: save tray preferences:", err)
			syncTrayMenu(items, selected)
			h.app.Dialog.Error().SetTitle(msg.savePrefsErrTitle).SetMessage(msg.savePrefsErrMsg).Show()
			return
		}
		selected = next
		syncTrayMenu(items, selected)
		refreshTray(sampler.Latest())
	}
	for _, option := range trayOptions {
		items[option.metric] = display.AddCheckbox(option.label(loc), selected&option.metric != 0).
			OnClick(func(*application.Context) {
				application.InvokeAsync(func() { applySelection(selected.toggle(option.metric)) })
			})
	}
	display.AddSeparator()
	display.Add(msg.keepAtLeastOne).SetEnabled(false)
	display.Add(msg.restoreDefaults).OnClick(func(*application.Context) {
		application.InvokeAsync(func() { applySelection(defaultTrayMetrics) })
	})
	syncTrayMenu(items, selected)
	menu.AddSeparator()
	loginDriver := newLoginItemDriver()
	var loginItem *application.MenuItem
	if updater.Status().State != update.Off {
		if loginDriver.available() {
			loginItem = menu.AddCheckbox(msg.openAtLogin, false)
			prompt := wailsLoginPrompt{app: h.app, driver: loginDriver, msg: msg}
			loginItem.OnClick(func(*application.Context) {
				application.InvokeAsync(func() { toggleLoginItem(loginItem, loginDriver, prompt, msg) })
			})
			syncLoginItem(loginItem, loginDriver, msg)
		}
		menu.Add(msg.checkForUpdates).OnClick(func(*application.Context) { updater.Check() })
		menu.AddSeparator()
	}
	menu.Add(msg.quitApp).OnClick(func(*application.Context) { h.app.Quit() })
	tray.SetMenu(menu)
	// System Settings can change login-item state out of band.
	tray.OnRightClick(func() {
		if h.panel != nil && h.panel.IsVisible() {
			h.panel.Hide()
		}
		syncLoginItem(loginItem, loginDriver, msg)
		tray.OpenMenu()
	})

	go sampler.Run(ctx)
	go func() {
		for s := range samples {
			application.InvokeAsync(func() { refreshTray(s) })
		}
	}()
	go updater.Run()
	return h.app.Run()
}

func syncTrayMenu(items map[trayMetrics]*application.MenuItem, selected trayMetrics) {
	for _, option := range trayOptions {
		checked := selected&option.metric != 0
		items[option.metric].SetChecked(checked).SetEnabled(selected != option.metric)
	}
}

// panelOptions is the dropdown: frameless, see-through so the page draws
// the design's rounded card and its shadow, shown on every Space without
// activating the app, and hidden when it loses focus or on Escape.
func panelOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:            "panel",
		Title:           "Pulsewise",
		URL:             "/",
		Width:           panelWidth,
		Height:          440,
		Hidden:          true,
		Frameless:       true,
		AlwaysOnTop:     true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		BackgroundType:  application.BackgroundTypeTransparent,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTransparent,
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary,
			WindowClass:      application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{NonActivating: true},
			// the page draws the shadow; a window shadow would box it
			DisableShadow: true,
		},
	}
}

const detailWindowName = "details"

func detailWindowOptions(path string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:            detailWindowName,
		Title:           "Pulsewise",
		Mac:             application.MacWindow{TitleBar: application.MacTitleBarHidden},
		URL:             path,
		Width:           720,
		Height:          620,
		MinWidth:        560,
		MinHeight:       440,
		InitialPosition: application.WindowCentered,
	}
}
