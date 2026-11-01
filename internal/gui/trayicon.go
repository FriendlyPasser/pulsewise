package gui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

// The menu bar icon shows the selected CPU, GPU, memory and disk metrics.
// Each bar fills from the bottom. The template image uses black with alpha,
// so macOS can tint it for light and dark menu bars.
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

func trayPresentation(s stats.Snapshot, selected trayMetrics) ([]byte, string) {
	var levels []float64
	var labels []string
	for _, option := range trayOptions {
		if selected&option.metric == 0 {
			continue
		}
		usage, available := option.read(s)
		levels = append(levels, max(usage, 0))
		value := "不可用"
		if available {
			value = fmt.Sprintf("%.0f%%", usage)
		}
		labels = append(labels, option.label+" "+value)
	}
	return trayPNG(levels), strings.Join(labels, "，")
}

// trayPNG centers one to four bars. Up to three retain the original 3-unit
// width and 1.5-unit gaps; four use narrower bars to fit the same canvas.
func trayPNG(levels []float64) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, trayCanvas, trayCanvas))
	width, gap := 3.0, 1.5
	if len(levels) == 4 {
		width, gap = 2.25, 1
	}
	span := float64(len(levels))*width + float64(len(levels)-1)*gap
	start := (14 - span) / 2
	for i, v := range levels {
		x := start + (width+gap)*float64(i)
		level := 12 * clamp(v, 0, 100) / 100
		// a sliver so an idle bar still shows it is a bar
		level = math.Max(level, 1.2)
		bar(img, x, 1, width, 12, trackAlpha)
		bar(img, x, 13-level, width, level, 1)
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
