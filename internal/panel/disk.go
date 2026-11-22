package panel

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
)

// DiskScanner provides on-demand directory large-file scans.
type DiskScanner interface {
	DefaultScopes() (defaultRoot string, presets []stats.DiskScopePreset)
	Status() stats.DiskScanStatus
	Start(root string) (stats.DiskScanStatus, error)
	Cancel() stats.DiskScanStatus
}

type diskCapabilities struct {
	ChooseDir bool `json:"chooseDir"`
	Reveal    bool `json:"reveal"`
}

type diskDetailResponse struct {
	Disk         stats.Disk              `json:"disk"`
	DefaultRoot  string                  `json:"defaultRoot"`
	Presets      []stats.DiskScopePreset `json:"presets"`
	Capabilities diskCapabilities        `json:"capabilities"`
	Scan         stats.DiskScanStatus    `json:"scan"`
}

func registerDisk(mux *http.ServeMux, s SnapshotSource, d DiskScanner, ui DiskUI) {
	mux.HandleFunc("GET /api/disk", func(rw http.ResponseWriter, _ *http.Request) {
		defaultRoot := ""
		presets := []stats.DiskScopePreset{}
		scan := stats.DiskScanStatus{
			State:          stats.DiskScanIdle,
			SkippedSamples: []string{},
			Files:          []stats.DiskFile{},
			Limit:          stats.DefaultDiskScanLimit,
		}
		if d != nil {
			var p []stats.DiskScopePreset
			defaultRoot, p = d.DefaultScopes()
			if p != nil {
				presets = p
			}
			scan = d.Status()
		}
		writeJSON(rw, diskDetailResponse{
			Disk:         s.Latest().Disk,
			DefaultRoot:  defaultRoot,
			Presets:      presets,
			Capabilities: diskCapabilities{ChooseDir: ui != nil, Reveal: ui != nil},
			Scan:         scan,
		})
	})

	mux.HandleFunc("POST /api/disk/scan", func(rw http.ResponseWriter, r *http.Request) {
		if d == nil {
			http.Error(rw, "disk scanner unavailable", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Root string `json:"root"`
		}
		if err := decodeOptionalJSON(r.Body, &body); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		status, err := d.Start(body.Root)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(rw, status)
	})

	mux.HandleFunc("POST /api/disk/cancel", func(rw http.ResponseWriter, _ *http.Request) {
		if d == nil {
			http.Error(rw, "disk scanner unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(rw, d.Cancel())
	})

	mux.HandleFunc("POST /api/disk/choose", func(rw http.ResponseWriter, r *http.Request) {
		if ui == nil {
			http.Error(rw, "directory picker unavailable", http.StatusNotImplemented)
			return
		}
		var body struct {
			Initial string `json:"initial"`
		}
		if err := decodeOptionalJSON(r.Body, &body); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		path, cancelled, err := ui.ChooseDiskDirectory(body.Initial)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(rw, struct {
			Path      string `json:"path"`
			Cancelled bool   `json:"cancelled"`
		}{Path: path, Cancelled: cancelled})
	})

	mux.HandleFunc("POST /api/disk/reveal", func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		if body.Path == "" || strings.ContainsRune(body.Path, 0) || !filepath.IsAbs(body.Path) {
			http.Error(rw, "invalid path", http.StatusBadRequest)
			return
		}
		target := filepath.Clean(body.Path)
		if d == nil || !isWithinRoot(d.Status().Root, target) {
			http.Error(rw, "path outside active scan scope", http.StatusBadRequest)
			return
		}
		if _, err := os.Lstat(target); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(rw, "file not found", http.StatusNotFound)
				return
			}
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if ui == nil {
			http.Error(rw, "reveal unavailable", http.StatusNotImplemented)
			return
		}
		if err := ui.RevealInFinder(target); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	})
}

func decodeOptionalJSON(r io.Reader, dst any) error {
	err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func isWithinRoot(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
