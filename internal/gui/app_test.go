package gui

import (
	"os"
	"path/filepath"
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

func TestDiskWindowOptions(t *testing.T) {
	opts := diskWindowOptions(messagesForLocale(panel.LocaleZhCN).diskWindowTitle)
	if opts.Name != "disk" || opts.Title != "磁盘大文件详情" || opts.URL != "/disk.html" {
		t.Fatalf("unexpected identity options: %+v", opts)
	}
	if opts.Width != 580 || opts.Height != 620 || opts.MinWidth != 460 || opts.MinHeight != 440 {
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

func TestResolveExistingDir(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveExistingDir(dir); got != dir {
		t.Fatalf("resolveExistingDir(%q) = %q, want %q", dir, got, dir)
	}
	if got := resolveExistingDir(filePath); got != "" {
		t.Fatalf("resolveExistingDir(file) = %q, want empty", got)
	}
	if got := resolveExistingDir("relative/path"); got != "" {
		t.Fatalf("resolveExistingDir(relative) = %q, want empty", got)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if got := resolveExistingDir("~"); got != filepath.Clean(home) {
			t.Fatalf("resolveExistingDir(~) = %q, want %q", got, filepath.Clean(home))
		}
	}
}
