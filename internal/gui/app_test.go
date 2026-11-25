package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestDetailWindowOptions(t *testing.T) {
	for _, path := range []string{"/memory.html", "/disk.html"} {
		opts := detailWindowOptions(messagesForLocale(panel.LocaleZhCN).detailWindowTitle, path)
		if opts.Name != "details" || opts.Title != "资源详情" || opts.URL != path {
			t.Fatalf("unexpected detail window options: %+v", opts)
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
