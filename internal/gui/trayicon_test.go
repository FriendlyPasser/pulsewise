package gui

import (
	"bytes"
	"image/png"
	"math/bits"
	"testing"

	"github.com/FriendlyPasser/pulsewise/internal/panel"
	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

func TestTooltipShowsDiskCapacity(t *testing.T) {
	snap := stats.Snapshot{Disk: stats.Disk{Total: 1000, Used: 750, Available: 250}}
	if _, tip := trayPresentation(snap, showDisk, panel.LocaleZhCN); tip != "硬盘 75%" {
		t.Fatalf("zh tooltip = %q", tip)
	}
	if _, tip := trayPresentation(stats.Snapshot{}, showDisk, panel.LocaleZhCN); tip != "硬盘 不可用" {
		t.Fatalf("zh unavailable tooltip = %q", tip)
	}
	if _, tip := trayPresentation(snap, showDisk, panel.LocaleEn); tip != "Disk 75%" {
		t.Fatalf("en tooltip = %q", tip)
	}
	if _, tip := trayPresentation(stats.Snapshot{}, showDisk, panel.LocaleEn); tip != "Disk unavailable" {
		t.Fatalf("en unavailable tooltip = %q", tip)
	}
}

func TestTrayPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayPNG([]float64{23, 41, 62})))
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

func TestTrayPresentationFollowsSelection(t *testing.T) {
	snap := stats.Snapshot{
		CPU: stats.CPU{Usage: 23}, GPU: stats.GPU{Usage: 41},
		Memory: stats.Memory{Total: 100, Used: 62},
		Disk:   stats.Disk{Total: 100, Used: 80, Available: 20},
	}
	for selected := trayMetrics(1); selected <= showCPU|showGPU|showMemory|showDisk; selected++ {
		data, _ := trayPresentation(snap, selected, panel.LocaleEn)
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		tracks, inside := 0, false
		for x := range trayCanvas {
			_, _, _, alpha := img.At(x, 20).RGBA()
			active := alpha > 0x2000
			if active && !inside {
				tracks++
			}
			inside = active
		}
		if want := bits.OnesCount8(uint8(selected)); tracks != want {
			t.Fatalf("selection %v has %d visible bars, want %d", selected, tracks, want)
		}
	}
	if _, tip := trayPresentation(snap, showCPU|showMemory|showDisk, panel.LocaleZhCN); tip != "CPU 23%，内存 62%，硬盘 80%" {
		t.Fatalf("zh selected tooltip = %q", tip)
	}
	if _, tip := trayPresentation(snap, showCPU|showMemory|showDisk, panel.LocaleEn); tip != "CPU 23%, Memory 62%, Disk 80%" {
		t.Fatalf("en selected tooltip = %q", tip)
	}
	snap.GPU.Usage = -1
	if _, tip := trayPresentation(snap, showGPU, panel.LocaleZhCN); tip != "GPU 不可用" {
		t.Fatalf("zh unavailable GPU tooltip = %q", tip)
	}
	if _, tip := trayPresentation(snap, showGPU, panel.LocaleEn); tip != "GPU unavailable" {
		t.Fatalf("en unavailable GPU tooltip = %q", tip)
	}
}
