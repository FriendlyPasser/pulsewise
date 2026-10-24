package gui

import (
	"bytes"
	"image/png"
	"testing"
)

func TestTrayPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayPNG(23, 41, 62)))
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
