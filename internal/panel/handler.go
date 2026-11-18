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
	Quit()
}

type SnapshotSource interface {
	Latest() stats.Snapshot
	MemoryDetail() stats.MemoryDetail
}

// Handler serves the panel and memory detail pages from embedded assets and
// their JSON API:
//
//	GET  /api/state        the latest sample
//	GET  /api/memory       per-application memory breakdown
//	POST /api/memory/open  opens the application memory detail window
//	POST /api/fit          {height}: the panel is sized to the page
//	POST /api/quit         quits the app
//
// Updates is optional. Without it, status is off and update commands are absent.
func Handler(w Host, s SnapshotSource, u Updates, loc Locale) http.Handler {
	tmpl := template.Must(template.ParseFS(assets, "assets/*.html"))
	msg := messagesForLocale(loc)
	indexHTML := renderHTML(tmpl, "index.html", msg)
	memoryHTML := renderHTML(tmpl, "memory.html", msg)

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
