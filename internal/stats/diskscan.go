package stats

import (
	"container/heap"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	DefaultDiskScanLimit   = 100
	maxSkippedDirSamples   = 5
	progressFlushInterval  = 150 * time.Millisecond
	progressFlushBatchSize = 512
)

var (
	ErrInvalidScanRoot  = errors.New("invalid scan root")
	ErrScanRootNotFound = errors.New("scan root does not exist")
	ErrScanRootNotDir   = errors.New("scan root is not a directory")
)

type DiskScanState string

const (
	DiskScanIdle      DiskScanState = "idle"
	DiskScanScanning  DiskScanState = "scanning"
	DiskScanCompleted DiskScanState = "completed"
	DiskScanCancelled DiskScanState = "cancelled"
	DiskScanError     DiskScanState = "error"
)

type DiskFile struct {
	Name        string `json:"name"`
	Dir         string `json:"dir"`
	Path        string `json:"path"`
	Size        uint64 `json:"size"`        // allocated on-disk bytes (st_blocks * 512 on Darwin)
	LogicalSize uint64 `json:"logicalSize"` // logical st_size bytes
}

type DiskScopePreset struct {
	ID   string `json:"id"`   // "downloads", "desktop", "documents", "home"
	Path string `json:"path"` // cleaned absolute path
}

type DiskScanStatus struct {
	ID             uint64        `json:"id"`
	State          DiskScanState `json:"state"`
	Root           string        `json:"root"`
	StartedAt      int64         `json:"startedAt,omitempty"`
	FinishedAt     int64         `json:"finishedAt,omitempty"`
	ElapsedMs      int64         `json:"elapsedMs"`
	ScannedFiles   int           `json:"scannedFiles"`
	ScannedDirs    int           `json:"scannedDirs"`
	ScannedBytes   uint64        `json:"scannedBytes"`
	SkippedDirs    int           `json:"skippedDirs"`
	SkippedSamples []string      `json:"skippedSamples"`
	Incomplete     bool          `json:"incomplete"`
	Truncated      bool          `json:"truncated"`
	Error          string        `json:"error,omitempty"`
	Files          []DiskFile    `json:"files"`
	Limit          int           `json:"limit"`
}

type regularFileStat struct {
	Allocated uint64
	Logical   uint64
	Dev       uint64
	Ino       uint64
	Nlink     uint64
}

type fileInode struct {
	dev uint64
	ino uint64
}

// DefaultDiskScopes returns the default starting folder (Downloads when
// available) and existing preset scan directories under home.
func DefaultDiskScopes(home string) (defaultRoot string, presets []DiskScopePreset) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			return "", []DiskScopePreset{}
		}
	}
	home = filepath.Clean(home)
	candidates := []DiskScopePreset{
		{ID: "downloads", Path: filepath.Join(home, "Downloads")},
		{ID: "desktop", Path: filepath.Join(home, "Desktop")},
		{ID: "documents", Path: filepath.Join(home, "Documents")},
		{ID: "home", Path: home},
	}
	presets = make([]DiskScopePreset, 0, len(candidates))
	for _, c := range candidates {
		info, err := os.Stat(c.Path)
		if err == nil && info.IsDir() {
			presets = append(presets, c)
		}
	}
	for _, p := range presets {
		if p.ID == "downloads" {
			return p.Path, presets
		}
	}
	if len(presets) > 0 {
		return presets[0].Path, presets
	}
	return home, presets
}

// ResolveScanRoot normalizes and validates a user-chosen scan directory.
func ResolveScanRoot(raw, defaultRoot, home string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		p = strings.TrimSpace(defaultRoot)
	}
	if p == "" || strings.ContainsRune(p, 0) {
		return "", ErrInvalidScanRoot
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil || home == "" {
				return "", ErrInvalidScanRoot
			}
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		return "", ErrInvalidScanRoot
	}
	clean := filepath.Clean(p)
	info, err := os.Stat(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrScanRootNotFound
		}
		return "", err
	}
	if !info.IsDir() {
		return "", ErrScanRootNotDir
	}
	return clean, nil
}

// DiskScanner runs one on-demand large-file directory scan at a time and
// keeps bounded top-N results plus live progress snapshots.
type DiskScanner struct {
	appCtx context.Context
	limit  int
	home   string

	mu          sync.Mutex
	cancel      context.CancelFunc
	status      DiskScanStatus
	startedTime time.Time
}

func NewDiskScanner(ctx context.Context) *DiskScanner {
	return newDiskScanner(ctx, DefaultDiskScanLimit, "")
}

func newDiskScanner(ctx context.Context, limit int, home string) *DiskScanner {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = DefaultDiskScanLimit
	}
	return &DiskScanner{
		appCtx: ctx,
		limit:  limit,
		home:   home,
		status: DiskScanStatus{
			State:          DiskScanIdle,
			SkippedSamples: []string{},
			Files:          []DiskFile{},
			Limit:          limit,
		},
	}
}

func (s *DiskScanner) DefaultScopes() (string, []DiskScopePreset) {
	return DefaultDiskScopes(s.home)
}

func (s *DiskScanner) Status() DiskScanStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clonedStatusLocked()
}

func (s *DiskScanner) clonedStatusLocked() DiskScanStatus {
	st := s.status
	if st.State == DiskScanScanning && !s.startedTime.IsZero() {
		st.ElapsedMs = max(time.Since(s.startedTime).Milliseconds(), 0)
	}
	if st.Files == nil {
		st.Files = []DiskFile{}
	} else {
		st.Files = slices.Clone(st.Files)
	}
	if st.SkippedSamples == nil {
		st.SkippedSamples = []string{}
	} else {
		st.SkippedSamples = slices.Clone(st.SkippedSamples)
	}
	return st
}

// Start validates root, supersedes any active scan, and starts scanning
// asynchronously. Validation failures preserve the previous scan status.
func (s *DiskScanner) Start(root string) (DiskScanStatus, error) {
	defaultRoot, _ := s.DefaultScopes()
	cleanRoot, err := ResolveScanRoot(root, defaultRoot, s.home)
	if err != nil {
		return s.Status(), err
	}

	now := time.Now()
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	gen := s.status.ID + 1
	scanCtx, scanCancel := context.WithCancel(s.appCtx)
	s.cancel = scanCancel
	s.startedTime = now
	s.status = DiskScanStatus{
		ID:             gen,
		State:          DiskScanScanning,
		Root:           cleanRoot,
		StartedAt:      now.UnixMilli(),
		SkippedSamples: []string{},
		Files:          []DiskFile{},
		Limit:          s.limit,
	}
	out := s.clonedStatusLocked()
	s.mu.Unlock()

	go s.runScan(scanCtx, scanCancel, gen, cleanRoot)
	return out, nil
}

// Cancel stops the active scan, if any, preserving partial progress.
func (s *DiskScanner) Cancel() DiskScanStatus {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.State != DiskScanScanning {
		return s.clonedStatusLocked()
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.status.State = DiskScanCancelled
	s.status.FinishedAt = now.UnixMilli()
	s.status.ElapsedMs = max(now.Sub(s.startedTime).Milliseconds(), 0)
	s.status.Incomplete = true
	return s.clonedStatusLocked()
}

func (s *DiskScanner) runScan(ctx context.Context, scanCancel context.CancelFunc, gen uint64, root string) {
	defer scanCancel()

	rootInfo, err := os.Stat(root)
	if err != nil {
		if ctx.Err() != nil {
			s.markAppCancelled(gen)
			return
		}
		s.markError(gen, root, err)
		return
	}
	rootDev, hasDev := statDevice(rootInfo)

	dirs := []string{root}
	top := make(diskFileHeap, 0, s.limit)
	var seenInodes map[fileInode]struct{}
	var scannedFiles, scannedDirs, skippedDirs, qualifying int
	var scannedBytes uint64
	skippedSamples := make([]string, 0, maxSkippedDirSamples)
	lastFlush := time.Now()
	sinceFlush := 0

	flushProgress := func() {
		now := time.Now()
		ranked := top.sortedCopy()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.status.ID != gen || s.status.State != DiskScanScanning {
			return
		}
		s.status.ElapsedMs = max(now.Sub(s.startedTime).Milliseconds(), 0)
		s.status.ScannedFiles = scannedFiles
		s.status.ScannedDirs = scannedDirs
		s.status.ScannedBytes = scannedBytes
		s.status.SkippedDirs = skippedDirs
		s.status.SkippedSamples = slices.Clone(skippedSamples)
		s.status.Incomplete = skippedDirs > 0
		s.status.Truncated = qualifying > s.limit
		s.status.Files = ranked
	}

	recordSkipped := func(dir string) {
		skippedDirs++
		if len(skippedSamples) < maxSkippedDirSamples {
			skippedSamples = append(skippedSamples, dir)
		}
	}

	for len(dirs) > 0 {
		if ctx.Err() != nil {
			s.markAppCancelled(gen)
			return
		}
		last := len(dirs) - 1
		dir := dirs[last]
		dirs = dirs[:last]

		entries, err := os.ReadDir(dir)
		if err != nil {
			if ctx.Err() != nil {
				s.markAppCancelled(gen)
				return
			}
			if dir == root {
				s.markError(gen, root, err)
				return
			}
			recordSkipped(dir)
			continue
		}
		scannedDirs++

		for _, entry := range entries {
			if ctx.Err() != nil {
				s.markAppCancelled(gen)
				return
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			childPath := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				// Avoid scanning the firmlinked Data volume twice when scanning "/".
				if root == "/" && childPath == "/System/Volumes/Data" {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						recordSkipped(childPath)
					}
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}
				if hasDev {
					if childDev, ok := statDevice(info); ok && childDev != rootDev {
						continue
					}
				}
				dirs = append(dirs, childPath)
				continue
			}
			if !entry.Type().IsRegular() {
				continue
			}
			info, err := entry.Info()
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if info.IsDir() {
				if root == "/" && childPath == "/System/Volumes/Data" {
					continue
				}
				if hasDev {
					if childDev, ok := statDevice(info); ok && childDev != rootDev {
						continue
					}
				}
				dirs = append(dirs, childPath)
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			fs, ok := statRegularFile(info)
			if !ok {
				continue
			}

			scannedFiles++
			if fs.Allocated > 0 {
				dup := false
				if fs.Nlink > 1 {
					if seenInodes == nil {
						seenInodes = make(map[fileInode]struct{})
					}
					inode := fileInode{dev: fs.Dev, ino: fs.Ino}
					if _, seen := seenInodes[inode]; seen {
						dup = true
					} else {
						seenInodes[inode] = struct{}{}
					}
				}
				if !dup {
					scannedBytes += fs.Allocated
					qualifying++
					candidate := DiskFile{
						Name:        entry.Name(),
						Dir:         dir,
						Path:        childPath,
						Size:        fs.Allocated,
						LogicalSize: fs.Logical,
					}
					if len(top) < s.limit {
						heap.Push(&top, candidate)
					} else if fileBetter(candidate, top[0]) {
						top[0] = candidate
						heap.Fix(&top, 0)
					}
				}
			}

			sinceFlush++
			if sinceFlush >= progressFlushBatchSize || time.Since(lastFlush) >= progressFlushInterval {
				flushProgress()
				sinceFlush = 0
				lastFlush = time.Now()
			}
		}
	}

	if ctx.Err() != nil {
		s.markAppCancelled(gen)
		return
	}

	now := time.Now()
	ranked := top.sortedCopy()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.ID != gen || s.status.State != DiskScanScanning {
		return
	}
	s.cancel = nil
	s.status.State = DiskScanCompleted
	s.status.FinishedAt = now.UnixMilli()
	s.status.ElapsedMs = max(now.Sub(s.startedTime).Milliseconds(), 0)
	s.status.ScannedFiles = scannedFiles
	s.status.ScannedDirs = scannedDirs
	s.status.ScannedBytes = scannedBytes
	s.status.SkippedDirs = skippedDirs
	s.status.SkippedSamples = slices.Clone(skippedSamples)
	s.status.Incomplete = skippedDirs > 0
	s.status.Truncated = qualifying > s.limit
	s.status.Files = ranked
}

func (s *DiskScanner) markAppCancelled(gen uint64) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.ID != gen || s.status.State != DiskScanScanning {
		return
	}
	s.cancel = nil
	s.status.State = DiskScanCancelled
	s.status.FinishedAt = now.UnixMilli()
	s.status.ElapsedMs = max(now.Sub(s.startedTime).Milliseconds(), 0)
	s.status.Incomplete = true
}

func (s *DiskScanner) markError(gen uint64, root string, err error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.ID != gen || s.status.State != DiskScanScanning {
		return
	}
	s.cancel = nil
	s.status.State = DiskScanError
	s.status.FinishedAt = now.UnixMilli()
	s.status.ElapsedMs = max(now.Sub(s.startedTime).Milliseconds(), 0)
	s.status.SkippedDirs = 1
	s.status.SkippedSamples = []string{root}
	s.status.Incomplete = true
	s.status.Error = err.Error()
}

func fileBetter(a, b DiskFile) bool {
	if a.Size != b.Size {
		return a.Size > b.Size
	}
	if a.LogicalSize != b.LogicalSize {
		return a.LogicalSize > b.LogicalSize
	}
	return a.Path < b.Path
}

type diskFileHeap []DiskFile

func (h diskFileHeap) Len() int           { return len(h) }
func (h diskFileHeap) Less(i, j int) bool { return fileBetter(h[j], h[i]) }
func (h diskFileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *diskFileHeap) Push(x any)        { *h = append(*h, x.(DiskFile)) }
func (h *diskFileHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (h diskFileHeap) sortedCopy() []DiskFile {
	out := slices.Clone(h)
	if out == nil {
		return []DiskFile{}
	}
	slices.SortFunc(out, func(a, b DiskFile) int {
		if fileBetter(a, b) {
			return -1
		}
		if fileBetter(b, a) {
			return 1
		}
		return 0
	})
	return out
}
