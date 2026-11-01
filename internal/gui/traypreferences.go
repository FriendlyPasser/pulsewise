package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

type trayMetrics uint8

const (
	showCPU trayMetrics = 1 << iota
	showGPU
	showMemory
	showDisk
	defaultTrayMetrics = showCPU | showGPU | showMemory
)

// This order is shared by the menu, saved preferences, bars and tooltip.
var trayOptions = [...]struct {
	metric trayMetrics
	key    string
	label  string
	read   func(stats.Snapshot) (float64, bool)
}{
	{showCPU, "cpu", "CPU", func(s stats.Snapshot) (float64, bool) { return s.CPU.Usage, true }},
	{showGPU, "gpu", "GPU", func(s stats.Snapshot) (float64, bool) { return s.GPU.Usage, s.GPU.Usage >= 0 }},
	{showMemory, "memory", "内存", func(s stats.Snapshot) (float64, bool) { return s.Memory.Usage(), s.Memory.Total > 0 }},
	{showDisk, "disk", "硬盘", func(s stats.Snapshot) (float64, bool) { return s.Disk.Usage(), s.Disk.Total > 0 }},
}

func (m trayMetrics) toggle(metric trayMetrics) trayMetrics {
	if next := m ^ metric; next != 0 {
		return next
	}
	return m // Keep the menu bar entry visible and useful.
}

type trayPreferences struct {
	Metrics []string `json:"metrics"`
}

func trayPreferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Pulsewise", "tray.json"), nil
}

func loadTrayMetrics(path string) (trayMetrics, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultTrayMetrics, nil
	}
	if err != nil {
		return defaultTrayMetrics, err
	}
	var prefs trayPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return defaultTrayMetrics, err
	}
	var selected trayMetrics
	for _, key := range prefs.Metrics {
		var metric trayMetrics
		for _, option := range trayOptions {
			if option.key == key {
				metric = option.metric
				break
			}
		}
		if metric == 0 || selected&metric != 0 {
			return defaultTrayMetrics, fmt.Errorf("unknown or duplicate tray metric %q", key)
		}
		selected |= metric
	}
	if selected == 0 {
		return defaultTrayMetrics, errors.New("select at least one tray metric")
	}
	return selected, nil
}

// Save in the same directory and rename so an interrupted write leaves the
// previous selection intact. Only the native UI thread writes preferences.
func saveTrayMetrics(path string, selected trayMetrics) error {
	if path == "" {
		return errors.New("configuration directory is unavailable")
	}
	prefs := trayPreferences{Metrics: make([]string, 0, len(trayOptions))}
	for _, option := range trayOptions {
		if selected&option.metric != 0 {
			prefs.Metrics = append(prefs.Metrics, option.key)
		}
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".tray-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
