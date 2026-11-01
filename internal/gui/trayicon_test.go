package gui

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

func TestTooltipShowsDiskCapacity(t *testing.T) {
	snap := stats.Snapshot{Disk: stats.Disk{Total: 1000, Used: 750, Available: 250}}
	if tip := tooltipOf(snap); !strings.HasSuffix(tip, "硬盘 75%") {
		t.Fatalf("tooltip = %q", tip)
	}
	if tip := tooltipOf(stats.Snapshot{}); !strings.HasSuffix(tip, "硬盘 不可用") {
		t.Fatalf("unavailable tooltip = %q", tip)
	}
}

func TestTrayPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayPNG(23, 41, 62, 80)))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != trayCanvas || b.Dy() != trayCanvas {
		t.Fatalf("size %v", b)
	}
	// the CPU bar's track is faint at its top, its fill solid at its foot
	_, _, _, top := img.At(11, 10).RGBA()
	_, _, _, foot := img.At(11, 33).RGBA()
	if top >= foot || foot < 0xf000 {
		t.Fatalf("track %x, fill %x", top, foot)
	}
}
