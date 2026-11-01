package panel

import (
	"net/http"

	"github.com/FriendlyPasser/pulsewise/internal/update"
)

// Updates is the update capability consumed by the panel. Restart installs and
// schedules a relaunch; the handler quits the host only after it succeeds.
type Updates interface {
	Status() update.Status
	Check()
	Restart() error
	OpenPage() error
}

func registerUpdates(mux *http.ServeMux, u Updates, quit func()) {
	mux.HandleFunc("GET /api/update", func(rw http.ResponseWriter, _ *http.Request) {
		if u == nil {
			writeJSON(rw, update.Status{State: update.Off})
			return
		}
		writeJSON(rw, u.Status())
	})
	if u == nil {
		return
	}
	mux.HandleFunc("POST /api/update/check", func(rw http.ResponseWriter, _ *http.Request) {
		u.Check()
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/update/restart", func(rw http.ResponseWriter, _ *http.Request) {
		if err := u.Restart(); err != nil {
			http.Error(rw, err.Error(), http.StatusConflict)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
		go quit()
	})
	mux.HandleFunc("POST /api/update/page", func(rw http.ResponseWriter, _ *http.Request) {
		if err := u.OpenPage(); err != nil {
			http.Error(rw, err.Error(), http.StatusConflict)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	})
}
