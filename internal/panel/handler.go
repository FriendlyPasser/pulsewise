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
	Quit()
}

type SnapshotSource interface {
	Latest() stats.Snapshot
}

// Handler serves the page from the embedded assets and its JSON API:
//
//	GET  /api/state  the latest sample
//	POST /api/fit    {height}: the panel is sized to the page
//	POST /api/quit   quits the app
//
// Updates is optional. Without it, status is off and update commands are absent.
func Handler(w Host, s SnapshotSource, u Updates, loc Locale) http.Handler {
	tmpl := template.Must(template.ParseFS(assets, "assets/index.html"))
	var page bytes.Buffer
	if err := tmpl.Execute(&page, messagesForLocale(loc)); err != nil {
		panic(err)
	}
	indexHTML := page.Bytes()
	serveIndex := func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write(indexHTML)
	}

	mux := http.NewServeMux()
	sub, _ := fs.Sub(assets, "assets")
	mux.HandleFunc("GET /{$}", serveIndex)
	mux.HandleFunc("GET /index.html", serveIndex)
	mux.Handle("/", http.FileServerFS(sub))
	mux.HandleFunc("GET /api/state", func(rw http.ResponseWriter, _ *http.Request) {
		writeJSON(rw, s.Latest())
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

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(rw).Encode(v)
}
