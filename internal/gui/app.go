// Package gui hosts the native menu bar, panel window, and application lifecycle.
package gui

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/FriendlyPasser/pulsewise/internal/stats"
	"github.com/FriendlyPasser/pulsewise/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// panelWidth is the design's 340px panel plus the room its shadow takes.
const (
	panelWidth  = 340 + 2*panelShadow
	panelShadow = 16
)

type host struct {
	app   *application.App
	panel *application.WebviewWindow
}

func (h *host) FitPanel(height int) {
	application.InvokeSync(func() { h.panel.SetSize(panelWidth, height) })
}

func (h *host) Quit() { h.app.Quit() }

// Run starts the app and blocks until it quits.
//
// It sets no app icon: a menu bar extra shows it nowhere, and Wails
// decodes the 1024px PNG into a bitmap that kept 32 MB, half the app's
// memory. The Finder's icon is the .app bundle's job (build/icon).
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampler := stats.NewSampler(time.Second)
	updater := update.New(ctx)

	h := &host{}
	h.app = application.New(application.Options{
		Name:        "Pulsewise",
		Description: "CPU, GPU, memory and disk capacity in the menu bar",
		Assets:      application.AssetOptions{Handler: panel.Handler(h, sampler, updater)},
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
	samples := sampler.Subscribe(ctx)
	snap := <-samples
	tray.SetTooltip(tooltipOf(snap))
	tray.SetTemplateIcon(iconOf(snap))
	tray.AttachWindow(h.panel).WindowOffset(6)

	// right-click: the menu (Wails' default with a window attached)
	menu := h.app.NewMenu()
	if updater.Status().State != update.Off {
		menu.Add("检查更新").OnClick(func(*application.Context) { updater.Check() })
		menu.AddSeparator()
	}
	menu.Add("退出 Pulsewise").OnClick(func(*application.Context) { h.app.Quit() })
	tray.SetMenu(menu)

	go sampler.Run(ctx)
	go func() {
		for s := range samples {
			png := iconOf(s)
			tip := tooltipOf(s)
			application.InvokeAsync(func() {
				tray.SetTemplateIcon(png)
				tray.SetTooltip(tip)
			})
		}
	}()
	go updater.Run()
	return h.app.Run()
}

func iconOf(s stats.Snapshot) []byte {
	return trayPNG(s.CPU.Usage, max(s.GPU.Usage, 0), s.Memory.Usage())
}

func tooltipOf(s stats.Snapshot) string {
	disk := "不可用"
	if s.Disk.Total > 0 {
		disk = fmt.Sprintf("%.0f%%", s.Disk.Usage())
	}
	return fmt.Sprintf("CPU %.0f%%，GPU %.0f%%，内存 %.0f%%，硬盘 %s", s.CPU.Usage, max(s.GPU.Usage, 0), s.Memory.Usage(), disk)
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
