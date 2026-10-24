// Package gui is the menu bar app: a tray icon that draws the machine's
// load, and a panel dropped under it. The panel is a small web page served
// from embedded assets, which reads Go over a JSON API on the same handler
// (no binding generator, no bundler), as magpie's is.
package gui

import (
	"fmt"
	"log"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
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
	tray  *application.SystemTray
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
	sampler := stats.NewSampler(time.Second)
	go sampler.Run()

	h := &host{}
	h.app = application.New(application.Options{
		Name:        "Pulsewise",
		Description: "CPU, GPU and memory in the menu bar",
		Assets:      application.AssetOptions{Handler: Handler(h, sampler)},
		// a menu bar extra: no Dock icon, no Command-Tab
		Mac:          application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		ErrorHandler: func(err error) { log.Println("pulsewise:", err) },
	})

	h.panel = h.app.Window.NewWithOptions(panelOptions())

	h.tray = h.app.SystemTray.New()
	h.tray.SetTooltip("Pulsewise")
	snap := sampler.Latest()
	h.tray.SetTemplateIcon(iconOf(snap))
	h.tray.AttachWindow(h.panel).WindowOffset(6)

	// right-click: the menu (Wails' default with a window attached)
	menu := h.app.NewMenu()
	menu.Add("退出 Pulsewise").OnClick(func(*application.Context) { h.app.Quit() })
	h.tray.SetMenu(menu)

	sampler.OnSample(func(s stats.Snapshot) {
		png := iconOf(s)
		tip := tooltipOf(s)
		application.InvokeAsync(func() {
			h.tray.SetTemplateIcon(png)
			h.tray.SetTooltip(tip)
		})
	})
	return h.app.Run()
}

func iconOf(s stats.Snapshot) []byte {
	mem := 0.0
	if s.Memory.Total > 0 {
		mem = float64(s.Memory.Used) / float64(s.Memory.Total) * 100
	}
	return trayPNG(s.CPU.Usage, max(s.GPU.Usage, 0), mem)
}

func tooltipOf(s stats.Snapshot) string {
	mem := 0.0
	if s.Memory.Total > 0 {
		mem = float64(s.Memory.Used) / float64(s.Memory.Total) * 100
	}
	return fmt.Sprintf("CPU %.0f%%，GPU %.0f%%，内存 %.0f%%", s.CPU.Usage, max(s.GPU.Usage, 0), mem)
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
