// Package panel serves the shared page and its HTTP API without a native UI dependency.
package panel

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

//go:embed assets
var assets embed.FS

// Host is the window and application control needed by the panel.
type Host interface {
	FitPanel(height int)
	OpenMemory()
	OpenDisk()
	Quit()
}

// DiskUI is optionally implemented by native hosts that support directory
// selection dialogs and revealing files in Finder.
type DiskUI interface {
	ChooseDiskDirectory(initial string) (path string, cancelled bool, err error)
	RevealInFinder(path string) error
}

type SnapshotSource interface {
	Latest() stats.Snapshot
	MemoryDetail() stats.MemoryDetail
}

// Handler serves the panel, memory detail, and disk large-file pages from
// embedded assets and their JSON API:
//
//	GET  /api/state        the latest sample
//	GET  /api/memory       per-application memory breakdown
//	POST /api/memory/open  opens the application memory detail window
//	GET  /api/disk         system disk capacity, presets, and latest large-file scan
//	POST /api/disk/open    opens the disk large-file detail window
//	POST /api/disk/scan    starts an on-demand large-file scan
//	POST /api/disk/cancel  cancels the running large-file scan
//	POST /api/disk/choose  opens a native directory picker (native host only)
//	POST /api/disk/reveal  reveals a scanned file in Finder (native host only)
//	POST /api/fit          {height}: the panel is sized to the page
//	POST /api/quit         quits the app
//
// Updates and DiskScanner are optional; DiskUI is optional via Host type assertion.
func Handler(w Host, s SnapshotSource, d DiskScanner, u Updates, loc Locale) http.Handler {
	tmpl := template.Must(template.ParseFS(assets, "assets/*.html"))
	msg := messagesForLocale(loc)
	indexHTML := renderHTML(tmpl, "index.html", msg)
	memoryHTML := renderHTML(tmpl, "memory.html", msg)
	diskHTML := renderHTML(tmpl, "disk.html", msg)

	serveHTML := func(body []byte) http.HandlerFunc {
		return func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = rw.Write(body)
		}
	}

	mux := http.NewServeMux()
	sub, _ := fs.Sub(assets, "assets")
	mux.HandleFunc("GET /{$}", serveHTML(indexHTML))
	mux.HandleFunc("GET /index.html", serveHTML(indexHTML))
	mux.HandleFunc("GET /memory", serveHTML(memoryHTML))
	mux.HandleFunc("GET /memory.html", serveHTML(memoryHTML))
	mux.HandleFunc("GET /disk", serveHTML(diskHTML))
	mux.HandleFunc("GET /disk.html", serveHTML(diskHTML))
	mux.Handle("/", http.FileServerFS(sub))
	mux.HandleFunc("GET /api/state", func(rw http.ResponseWriter, _ *http.Request) {
		writeJSON(rw, s.Latest())
	})
	mux.HandleFunc("GET /api/memory", func(rw http.ResponseWriter, _ *http.Request) {
		writeJSON(rw, s.MemoryDetail())
	})
	mux.HandleFunc("POST /api/memory/open", func(rw http.ResponseWriter, _ *http.Request) {
		w.OpenMemory()
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/disk/open", func(rw http.ResponseWriter, _ *http.Request) {
		w.OpenDisk()
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/fit", func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			Height int `json:"height"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Height <= 0 {
			http.Error(rw, "height", http.StatusBadRequest)
			return
		}
		w.FitPanel(body.Height)
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/quit", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
		go w.Quit()
	})
	diskUI, _ := w.(DiskUI)
	registerDisk(mux, s, d, diskUI)
	registerUpdates(mux, u, w.Quit)
	return mux
}

func renderHTML(tmpl *template.Template, name string, msg panelMessages) []byte {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, msg); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(rw).Encode(v)
}
