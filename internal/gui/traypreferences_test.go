package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestTrayPreferencesDefaultAndInvalidFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.json")
	if got, err := loadTrayMetrics(path); err != nil || got != defaultTrayMetrics {
		t.Fatalf("missing preferences = %v, %v", got, err)
	}
	for _, data := range []string{
		`broken`, `null`, `{}`, `{"metrics":[]}`,
		`{"metrics":["cpu","cpu"]}`, `{"metrics":["cpu","network"]}`,
	} {
		t.Run(data, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadTrayMetrics(path); err == nil || got != defaultTrayMetrics {
				t.Fatalf("invalid preferences = %v, %v", got, err)
			}
		})
	}
	if got, err := loadTrayMetrics(filepath.Dir(path)); err == nil || got != defaultTrayMetrics {
		t.Fatalf("unreadable preferences = %v, %v", got, err)
	}
}

func TestTrayPreferencesRememberAllNonemptySelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Pulsewise", "tray.json")
	for selected := trayMetrics(1); selected <= showCPU|showGPU|showMemory|showDisk; selected++ {
		if err := saveTrayMetrics(path, selected); err != nil {
			t.Fatal(err)
		}
		if got, err := loadTrayMetrics(path); err != nil || got != selected {
			t.Fatalf("saved %v, loaded %v, error %v", selected, got, err)
		}
	}
	if err := saveTrayMetrics(path, showDisk|showCPU); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var prefs trayPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prefs.Metrics, []string{"cpu", "disk"}) {
		t.Fatalf("saved order = %v", prefs.Metrics)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 || files[0].Name() != "tray.json" {
		t.Fatalf("temporary preferences were left behind: %v, %v", files, err)
	}
}

func TestTrayPreferencesSaveReportsFailure(t *testing.T) {
	if err := saveTrayMetrics("", defaultTrayMetrics); err == nil {
		t.Fatal("saving without a configuration directory succeeded")
	}
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveTrayMetrics(filepath.Join(path, "tray.json"), showDisk); err == nil {
		t.Fatal("saving inside a regular file succeeded")
	}
}

func TestTrayMetricsAlwaysRetainOneSelection(t *testing.T) {
	for selected := trayMetrics(1); selected <= showCPU|showGPU|showMemory|showDisk; selected++ {
		for _, option := range trayOptions {
			next := selected.toggle(option.metric)
			if next == 0 || next&^option.metric != selected&^option.metric {
				t.Fatalf("toggling %v from %v produced %v", option.metric, selected, next)
			}
		}
	}
}

func TestTrayMenuReflectsSelectionAndProtectsLastItem(t *testing.T) {
	for _, loc := range []panel.Locale{panel.LocaleZhCN, panel.LocaleEn} {
		t.Run(string(loc), func(t *testing.T) {
			items := make(map[trayMetrics]*application.MenuItem, len(trayOptions))
			for _, option := range trayOptions {
				items[option.metric] = application.NewMenuItemCheckbox(option.label(loc), false)
			}
			for selected := trayMetrics(1); selected <= showCPU|showGPU|showMemory|showDisk; selected++ {
				syncTrayMenu(items, selected)
				for _, option := range trayOptions {
					item := items[option.metric]
					if item.Checked() != (selected&option.metric != 0) || item.Enabled() != (selected != option.metric) {
						t.Fatalf("selection %v, item %s: checked=%v enabled=%v", selected, option.key, item.Checked(), item.Enabled())
					}
				}
			}
		})
	}
}

func TestTrayOptionLabels(t *testing.T) {
	want := map[panel.Locale][]string{
		panel.LocaleZhCN: {"CPU", "GPU", "内存", "硬盘"},
		panel.LocaleEn:   {"CPU", "GPU", "Memory", "Disk"},
	}
	for loc, labels := range want {
		for i, option := range trayOptions {
			if got := option.label(loc); got != labels[i] {
				t.Fatalf("loc %s option %s label = %q, want %q", loc, option.key, got, labels[i])
			}
		}
	}
}
