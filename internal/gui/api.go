package gui

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
	"github.com/FriendlyPasser/pulsewise/internal/update"
)

//go:embed assets
var assets embed.FS

// Windows is what the API asks of the native host.
type Windows interface {
	FitPanel(height int)
	Quit()
	RestartToUpdate() error
}

// Handler serves the page from the embedded assets and its JSON API:
//
//	GET  /api/state  the latest sample
//	POST /api/fit    {height}: the panel is sized to the page
//	POST /api/quit   quits the app
//
// and, with an updater (nil in -web mode):
//
//	GET  /api/update          where an update stands (update.Status)
//	POST /api/update/check    asks the feed now
//	POST /api/update/restart  puts the downloaded version in and restarts
//	POST /api/update/page     opens the release page
func Handler(w Windows, s *stats.Sampler, u *update.Updater) http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(assets, "assets")
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
	mux.HandleFunc("GET /api/update", func(rw http.ResponseWriter, _ *http.Request) {
		if u == nil {
			writeJSON(rw, update.Status{State: update.Off})
			return
		}
		writeJSON(rw, u.Status())
	})
	if u != nil {
		mux.HandleFunc("POST /api/update/check", func(rw http.ResponseWriter, _ *http.Request) {
			u.Check()
			rw.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("POST /api/update/restart", func(rw http.ResponseWriter, _ *http.Request) {
			if err := w.RestartToUpdate(); err != nil {
				http.Error(rw, err.Error(), http.StatusConflict)
				return
			}
			rw.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("POST /api/update/page", func(rw http.ResponseWriter, _ *http.Request) {
			if err := u.OpenPage(); err != nil {
				http.Error(rw, err.Error(), http.StatusConflict)
				return
			}
			rw.WriteHeader(http.StatusNoContent)
		})
	}
	return mux
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(rw).Encode(v)
}

// web stands in for the native host when the page is served to a browser:
// there is no panel to fit, and quitting ends the process.
type web struct{ quit chan struct{} }

func (web) FitPanel(int) {}
func (w web) Quit()      { close(w.quit) }
func (web) RestartToUpdate() error {
	return errors.New("the browser's panel doesn't update the app")
}

// ServeWeb serves the panel to a browser at addr (pulsewise -web), for
// working on the page without the menu bar.
func ServeWeb(addr string) error {
	s := stats.NewSampler(time.Second)
	go s.Run()
	w := web{quit: make(chan struct{})}
	srv := &http.Server{Addr: addr, Handler: Handler(w, s, nil)}
	go func() { <-w.quit; _ = srv.Close() }()
	log.Printf("pulsewise: serving the panel at http://%s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
