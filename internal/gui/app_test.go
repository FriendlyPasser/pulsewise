package gui

import (
	"testing"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMemoryWindowOptions(t *testing.T) {
	opts := memoryWindowOptions(messagesForLocale(panel.LocaleZhCN).memoryWindowTitle)
	if opts.Name != "memory" || opts.Title != "应用内存详情" || opts.URL != "/memory.html" {
		t.Fatalf("unexpected identity options: %+v", opts)
	}
	if opts.Width != 460 || opts.Height != 560 || opts.MinWidth != 380 || opts.MinHeight != 400 {
		t.Fatalf("unexpected size options: %+v", opts)
	}
	if opts.InitialPosition != application.WindowCentered {
		t.Fatalf("InitialPosition = %v, want WindowCentered", opts.InitialPosition)
	}
	if opts.Frameless || opts.HideOnFocusLost || opts.DisableResize {
		t.Fatalf("detail window should be standard resizable window: %+v", opts)
	}
	if opts.Mac.WindowClass != 0 {
		t.Fatalf("WindowClass = %v, want standard window", opts.Mac.WindowClass)
	}
}
