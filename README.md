# Pulsewise

CPU, GPU and memory in the macOS menu bar. Click the three bars to drop the panel; right-click for Quit.

Built like [magpie](https://github.com/yetone/magpie): Go + Wails v3 for the tray and the panel window, and one small page (`internal/gui/assets`, no framework or bundler) that reads Go over a JSON API on the same handler.

| Path | What |
| --- | --- |
| `internal/stats` | Samples once a second through cgo: Mach CPU ticks, `host_statistics64` memory pages, sysctl swap, IOKit `IOAccelerator` GPU utilization. No root. |
| `internal/gui/app.go` | The tray, its template icon redrawn each sample, and the panel window. |
| `internal/gui/api.go` | Serves the page; `GET /api/state`, `POST /api/fit`, `POST /api/quit`. |
| `internal/gui/trayicon.go` | Draws the menu bar icon (design style A) as a 44px template PNG. |
| `build/icon` | App icon (colour, plus the two monochrome variants). |

```sh
go build -o bin/pulsewise .
./bin/pulsewise                       # menu bar app
./bin/pulsewise -web 127.0.0.1:8765   # the panel in a browser, for working on the page
```
