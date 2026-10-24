package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// The menu bar icon is the design's style A, "三柱": three bars for CPU,
// GPU and memory, each a track with its level filled from the bottom. It
// is a template image (black with alpha), so the Mac tints it for a dark
// or light menu bar, as the design asks.
//
// Wails sizes a tray image to the bar's thickness (22pt), square, so the
// canvas is 22pt at 2x and the icon sits in its middle 16pt, as in the
// design's 28px-tall bar.
const (
	trayCanvas = 44   // px: 22pt at 2x
	trayIcon   = 32.0 // px: 16pt at 2x
	trayUnit   = trayIcon / 14
	trayInset  = (trayCanvas - trayIcon) / 2
	trackAlpha = 0.28 // the track, as #48484E on #1B1B1E reads
)

// trayPNG draws the three bars at cpu, gpu and mem, each 0–100.
func trayPNG(cpu, gpu, mem float64) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, trayCanvas, trayCanvas))
	for i, v := range []float64{cpu, gpu, mem} {
		x := 1 + 4.5*float64(i) // the design's bars at x = 1, 5.5, 10, 3 wide
		level := 12 * clamp(v, 0, 100) / 100
		// a sliver so an idle bar still shows it is a bar
		level = math.Max(level, 1.2)
		bar(img, x, 1, 3, 12, trackAlpha)
		bar(img, x, 13-level, 3, level, 1)
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// bar fills a rounded rect given in the design's 14-unit space, over what
// is there, with antialiased edges.
func bar(img *image.NRGBA, x, y, w, h, alpha float64) {
	r := math.Min(1, math.Min(w, h)/2) * trayUnit
	x0, y0 := trayInset+x*trayUnit, trayInset+y*trayUnit
	x1, y1 := x0+w*trayUnit, y0+h*trayUnit
	for py := int(y0) - 1; py <= int(y1)+1; py++ {
		for px := int(x0) - 1; px <= int(x1)+1; px++ {
			c := coverage(float64(px)+0.5, float64(py)+0.5, x0, y0, x1, y1, r)
			if c <= 0 {
				continue
			}
			a := c * alpha
			old := float64(img.NRGBAAt(px, py).A) / 255
			out := a + old*(1-a)
			img.SetNRGBA(px, py, color.NRGBA{A: uint8(math.Round(out * 255))})
		}
	}
}

// coverage is how much of the pixel at (px, py) the rounded rect covers,
// from its signed distance.
func coverage(px, py, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	dx, dy := math.Abs(px-cx)-hx, math.Abs(py-cy)-hy
	outside := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	inside := math.Min(math.Max(dx, dy), 0)
	d := outside + inside - r
	return clamp(0.5-d, 0, 1)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
