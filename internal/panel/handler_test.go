package panel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
	"github.com/FriendlyPasser/pulsewise/internal/update"
)

type fixedSnapshots struct {
	snapshot stats.Snapshot
	detail   stats.MemoryDetail
}

func (s fixedSnapshots) Latest() stats.Snapshot           { return s.snapshot }
func (s fixedSnapshots) MemoryDetail() stats.MemoryDetail { return s.detail }

type testHost struct {
	height     int
	openMemory int
	openDisk   int
	quit       chan struct{}
}

func (h *testHost) FitPanel(height int) { h.height = height }
func (h *testHost) OpenMemory()         { h.openMemory++ }
func (h *testHost) OpenDisk()           { h.openDisk++ }
func (h *testHost) Quit()               { h.quit <- struct{}{} }

type testNativeHost struct {
	*testHost
	chooseInitial string
	choosePath    string
	chooseCancel  bool
	chooseErr     error
	revealed      []string
	revealErr     error
}

func (h *testNativeHost) ChooseDiskDirectory(initial string) (string, bool, error) {
	h.chooseInitial = initial
	return h.choosePath, h.chooseCancel, h.chooseErr
}

func (h *testNativeHost) RevealInFinder(path string) error {
	h.revealed = append(h.revealed, path)
	return h.revealErr
}

type testDiskScanner struct {
	defaultRoot string
	presets     []stats.DiskScopePreset
	status      stats.DiskScanStatus
	startRoot   string
	startErr    error
	cancelCalls int
}

func (d *testDiskScanner) DefaultScopes() (string, []stats.DiskScopePreset) {
	return d.defaultRoot, d.presets
}

func (d *testDiskScanner) Status() stats.DiskScanStatus { return d.status }

func (d *testDiskScanner) Start(root string) (stats.DiskScanStatus, error) {
	d.startRoot = root
	if d.startErr != nil {
		return d.status, d.startErr
	}
	return d.status, nil
}

func (d *testDiskScanner) Cancel() stats.DiskScanStatus {
	d.cancelCalls++
	return d.status
}

type testUpdates struct {
	status     update.Status
	checks     int
	restarts   int
	pages      int
	restartErr error
	pageErr    error
}

func (u *testUpdates) Status() update.Status { return u.status }
func (u *testUpdates) Check()                { u.checks++ }
func (u *testUpdates) Restart() error        { u.restarts++; return u.restartErr }
func (u *testUpdates) OpenPage() error       { u.pages++; return u.pageErr }

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rw
}

func TestStateContract(t *testing.T) {
	want := stats.Snapshot{
		CPU: stats.CPU{Usage: 32, History: []float64{21, 32}},
		GPU: stats.GPU{Usage: -1}, Memory: stats.Memory{Total: 1024, Used: 512}, At: 1234,
		Disk: stats.Disk{Total: 1000, Used: 750, Available: 250},
	}
	h := Handler(&testHost{}, fixedSnapshots{snapshot: want}, nil, nil, LocaleZhCN)
	rw := request(h, "GET", "/api/state", "")
	var got stats.Snapshot
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rw.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("response %d: %+v", rw.Code, got)
	}
	if rw.Header().Get("Content-Type") != "application/json" || rw.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", rw.Header())
	}
}

func TestMemoryDetailContractAndOpen(t *testing.T) {
	want := stats.MemoryDetail{
		Memory: stats.Memory{Total: 16 << 30, Used: 8 << 30, App: 4 << 30, Wired: 2 << 30, Compressed: 2 << 30},
		Apps: []stats.AppMemory{
			{
				Name:   "Google Chrome",
				Memory: 3 << 30,
				Processes: []stats.ProcessMemory{
					{PID: 10, Name: "Google Chrome", Memory: 1 << 30},
					{PID: 11, Name: "Google Chrome Helper", Memory: 2 << 30},
				},
			},
		},
		Unattributed:       []stats.ProcessMemory{{PID: 20, Name: "sharingd", Memory: 64 << 20}},
		UnattributedMemory: 64 << 20,
		At:                 5678,
	}
	host := &testHost{}
	h := Handler(host, fixedSnapshots{detail: want}, nil, nil, LocaleEn)

	rw := request(h, "GET", "/api/memory", "")
	var got stats.MemoryDetail
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rw.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("response %d: %+v", rw.Code, got)
	}
	if rw.Header().Get("Content-Type") != "application/json" || rw.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", rw.Header())
	}

	openRW := request(h, "POST", "/api/memory/open", "")
	if openRW.Code != http.StatusNoContent || host.openMemory != 1 {
		t.Fatalf("open status %d, calls %d", openRW.Code, host.openMemory)
	}
}

func TestFitValidatesAtHTTPBoundary(t *testing.T) {
	for _, body := range []string{`{}`, `{"height":0}`, `{"height":-1}`, `{"height":"440"}`, `broken`} {
		t.Run(body, func(t *testing.T) {
			host := &testHost{}
			rw := request(Handler(host, fixedSnapshots{}, nil, nil, LocaleEn), "POST", "/api/fit", body)
			if rw.Code != http.StatusBadRequest || host.height != 0 {
				t.Fatalf("response %d, height %d", rw.Code, host.height)
			}
		})
	}
	host := &testHost{}
	rw := request(Handler(host, fixedSnapshots{}, nil, nil, LocaleEn), "POST", "/api/fit", `{"height":440}`)
	if rw.Code != http.StatusNoContent || host.height != 440 {
		t.Fatalf("response %d, height %d", rw.Code, host.height)
	}
}

func TestEmbeddedAssets(t *testing.T) {
	h := Handler(&testHost{}, fixedSnapshots{}, nil, nil, LocaleZhCN)
	for path, text := range map[string]string{
		"/":            "memMetric",
		"/index.html":  "cpuValue",
		"/memory":      "appList",
		"/memory.html": "unattributedList",
		"/disk":        "fileList",
		"/disk.html":   "scanRootInput",
		"/app.js":      "/api/state",
		"/memory.js":   "/api/memory",
		"/disk.js":     "/api/disk",
		"/app.css":     "--panel",
	} {
		rw := request(h, "GET", path, "")
		if rw.Code != http.StatusOK || !strings.Contains(rw.Body.String(), text) {
			t.Fatalf("asset %s: status %d", path, rw.Code)
		}
	}
}

func TestLocalizedMemoryHTML(t *testing.T) {
	cases := []struct {
		name    string
		loc     Locale
		want    []string
		notWant []string
	}{
		{
			name:    "simplified chinese",
			loc:     LocaleZhCN,
			want:    []string{`<html lang="zh-CN">`, "应用内存详情", "应用占用", "未归属应用的进程", "phys_footprint"},
			notWant: []string{`<html lang="en">`, "App Memory Details", "Unattributed Processes"},
		},
		{
			name:    "english",
			loc:     LocaleEn,
			want:    []string{`<html lang="en">`, "App Memory Details", "Applications", "Unattributed Processes", "phys_footprint"},
			notWant: []string{`<html lang="zh-CN">`, "应用内存详情", "未归属应用的进程"},
		},
		{
			name:    "unknown locale defaults to english",
			loc:     Locale("fr"),
			want:    []string{`<html lang="en">`, "App Memory Details", "Unattributed Processes"},
			notWant: []string{`<html lang="zh-CN">`, "应用内存详情"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler(&testHost{}, fixedSnapshots{}, nil, nil, tc.loc)
			for _, path := range []string{"/memory", "/memory.html"} {
				rw := request(h, "GET", path, "")
				if rw.Code != http.StatusOK {
					t.Fatalf("%s %s: status %d", tc.name, path, rw.Code)
				}
				if ct := rw.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
					t.Fatalf("%s %s: Content-Type = %q", tc.name, path, ct)
				}
				body := rw.Body.String()
				for _, s := range tc.want {
					if !strings.Contains(body, s) {
						t.Errorf("%s %s: missing %q", tc.name, path, s)
					}
				}
				for _, s := range tc.notWant {
					if strings.Contains(body, s) {
						t.Errorf("%s %s: unexpected %q", tc.name, path, s)
					}
				}
			}
		})
	}
}

func TestLocalizedPanelHTML(t *testing.T) {
	cases := []struct {
		name    string
		loc     Locale
		want    []string
		notWant []string
	}{
		{
			name:    "simplified chinese",
			loc:     LocaleZhCN,
			want:    []string{`<html lang="zh-CN">`, "用户", "系统", "空闲", "内存", "硬盘", "系统磁盘", "容量暂不可用", "退出"},
			notWant: []string{`<html lang="en">`, "Memory", "System Disk", "Capacity unavailable", "Quit"},
		},
		{
			name:    "english",
			loc:     LocaleEn,
			want:    []string{`<html lang="en">`, "User", "Sys", "Idle", "Memory", "Disk", "System Disk", "Capacity unavailable", "Quit"},
			notWant: []string{`<html lang="zh-CN">`, "内存", "硬盘", "系统磁盘", "容量暂不可用", "退出"},
		},
		{
			name:    "unknown locale defaults to english",
			loc:     Locale("fr"),
			want:    []string{`<html lang="en">`, "Memory", "System Disk", "Quit"},
			notWant: []string{`<html lang="zh-CN">`, "内存", "退出"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler(&testHost{}, fixedSnapshots{}, nil, nil, tc.loc)
			for _, path := range []string{"/", "/index.html"} {
				rw := request(h, "GET", path, "")
				if rw.Code != http.StatusOK {
					t.Fatalf("%s %s: status %d", tc.name, path, rw.Code)
				}
				if ct := rw.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
					t.Fatalf("%s %s: Content-Type = %q", tc.name, path, ct)
				}
				body := rw.Body.String()
				for _, s := range tc.want {
					if !strings.Contains(body, s) {
						t.Errorf("%s %s: missing %q", tc.name, path, s)
					}
				}
				for _, s := range tc.notWant {
					if strings.Contains(body, s) {
						t.Errorf("%s %s: unexpected %q", tc.name, path, s)
					}
				}
			}
		})
	}
}

func TestBrowserHasNoUpdateCommands(t *testing.T) {
	h := Handler(&testHost{}, fixedSnapshots{}, nil, nil, LocaleEn)
	rw := request(h, "GET", "/api/update", "")
	var status update.Status
	if err := json.Unmarshal(rw.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.State != update.Off {
		t.Fatalf("status = %+v", status)
	}
	for _, action := range []string{"check", "restart", "page"} {
		if rw := request(h, "POST", "/api/update/"+action, ""); rw.Code != http.StatusNotFound {
			t.Fatalf("browser update %s: status %d", action, rw.Code)
		}
	}
}

func TestUpdateCommands(t *testing.T) {
	u := &testUpdates{status: update.Status{State: update.Ready, Latest: "0.2.0"}}
	host := &testHost{quit: make(chan struct{}, 1)}
	h := Handler(host, fixedSnapshots{}, nil, u, LocaleEn)
	rw := request(h, "GET", "/api/update", "")
	var got update.Status
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil || got != u.status {
		t.Fatalf("status = %+v, error %v", got, err)
	}
	for _, path := range []string{"check", "page", "restart"} {
		if rw := request(h, "POST", "/api/update/"+path, ""); rw.Code != http.StatusNoContent {
			t.Fatalf("%s: status %d", path, rw.Code)
		}
	}
	select {
	case <-host.quit:
	case <-time.After(time.Second):
		t.Fatal("a successful restart did not quit the host")
	}
	if u.checks != 1 || u.pages != 1 || u.restarts != 1 {
		t.Fatalf("calls = %+v", u)
	}
}

func TestRestartFailureKeepsHostRunning(t *testing.T) {
	host := &testHost{quit: make(chan struct{}, 1)}
	u := &testUpdates{restartErr: errors.New("install failed"), pageErr: errors.New("no release page")}
	h := Handler(host, fixedSnapshots{}, nil, u, LocaleEn)
	for _, action := range []string{"restart", "page"} {
		if rw := request(h, "POST", "/api/update/"+action, ""); rw.Code != http.StatusConflict {
			t.Fatalf("%s: status %d", action, rw.Code)
		}
	}
	select {
	case <-host.quit:
		t.Fatal("a failed restart quit the host")
	default:
	}
}

func TestQuit(t *testing.T) {
	host := &testHost{quit: make(chan struct{}, 1)}
	rw := request(Handler(host, fixedSnapshots{}, nil, nil, LocaleEn), "POST", "/api/quit", "")
	if rw.Code != http.StatusNoContent {
		t.Fatalf("status %d", rw.Code)
	}
	select {
	case <-host.quit:
	case <-time.After(time.Second):
		t.Fatal("quit was not delivered")
	}
}

func TestDiskDetailAPIAndNativeCapabilities(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "big.bin")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(t.TempDir(), "other.bin")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := stats.Snapshot{Disk: stats.Disk{Total: 1000, Used: 400, Available: 600}}
	scanner := &testDiskScanner{
		defaultRoot: root,
		presets:     []stats.DiskScopePreset{{ID: "downloads", Path: root}},
		status: stats.DiskScanStatus{
			ID:             2,
			State:          stats.DiskScanCompleted,
			Root:           root,
			ScannedFiles:   1,
			ScannedDirs:    1,
			ScannedBytes:   5,
			SkippedSamples: []string{},
			Files:          []stats.DiskFile{{Name: "big.bin", Dir: root, Path: filePath, Size: 5, LogicalSize: 5}},
			Limit:          100,
		},
	}

	// Browser host (no DiskUI): capabilities false, choose/reveal return 501 after boundary checks.
	browserHost := &testHost{}
	bh := Handler(browserHost, fixedSnapshots{snapshot: snap}, scanner, nil, LocaleEn)

	openRW := request(bh, "POST", "/api/disk/open", "")
	if openRW.Code != http.StatusNoContent || browserHost.openDisk != 1 {
		t.Fatalf("open status %d, calls %d", openRW.Code, browserHost.openDisk)
	}

	stateRW := request(bh, "GET", "/api/disk", "")
	if stateRW.Code != http.StatusOK || stateRW.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /api/disk status=%d headers=%v", stateRW.Code, stateRW.Header())
	}
	var got diskDetailResponse
	if err := json.Unmarshal(stateRW.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.DefaultRoot != root || got.Capabilities.ChooseDir || got.Capabilities.Reveal || len(got.Presets) != 1 || got.Scan.ID != 2 {
		t.Fatalf("unexpected /api/disk payload: %+v", got)
	}

	if rw := request(bh, "POST", "/api/disk/choose", `{}`); rw.Code != http.StatusNotImplemented {
		t.Fatalf("browser choose status = %d, want 501", rw.Code)
	}
	if rw := request(bh, "POST", "/api/disk/reveal", `{"path":`+jsonQuote(filePath)+`}`); rw.Code != http.StatusNotImplemented {
		t.Fatalf("browser reveal status = %d, want 501", rw.Code)
	}

	// Native host (implements DiskUI).
	nativeHost := &testNativeHost{
		testHost:   &testHost{},
		choosePath: root,
	}
	nh := Handler(nativeHost, fixedSnapshots{snapshot: snap}, scanner, nil, LocaleZhCN)

	nativeStateRW := request(nh, "GET", "/api/disk", "")
	if err := json.Unmarshal(nativeStateRW.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Capabilities.ChooseDir || !got.Capabilities.Reveal {
		t.Fatalf("native host capabilities = %+v", got.Capabilities)
	}

	// Scan start & validation errors.
	scanRW := request(nh, "POST", "/api/disk/scan", `{"root":`+jsonQuote(root)+`}`)
	if scanRW.Code != http.StatusOK || scanner.startRoot != root {
		t.Fatalf("scan status=%d root=%q", scanRW.Code, scanner.startRoot)
	}
	if rw := request(nh, "POST", "/api/disk/scan", `{broken`); rw.Code != http.StatusBadRequest {
		t.Fatalf("malformed scan JSON status=%d", rw.Code)
	}
	scanner.startErr = stats.ErrInvalidScanRoot
	if rw := request(nh, "POST", "/api/disk/scan", `{"root":"rel"}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("invalid scan root status=%d", rw.Code)
	}
	scanner.startErr = nil

	// Cancel.
	cancelRW := request(nh, "POST", "/api/disk/cancel", "")
	if cancelRW.Code != http.StatusOK || scanner.cancelCalls != 1 {
		t.Fatalf("cancel status=%d calls=%d", cancelRW.Code, scanner.cancelCalls)
	}

	// Choose folder (picked and cancelled).
	chooseRW := request(nh, "POST", "/api/disk/choose", `{"initial":`+jsonQuote(root)+`}`)
	if chooseRW.Code != http.StatusOK || nativeHost.chooseInitial != root {
		t.Fatalf("choose status=%d initial=%q", chooseRW.Code, nativeHost.chooseInitial)
	}
	var chooseBody struct {
		Path      string `json:"path"`
		Cancelled bool   `json:"cancelled"`
	}
	if err := json.Unmarshal(chooseRW.Body.Bytes(), &chooseBody); err != nil || chooseBody.Path != root || chooseBody.Cancelled {
		t.Fatalf("choose body = %+v, err=%v", chooseBody, err)
	}
	nativeHost.choosePath = ""
	nativeHost.chooseCancel = true
	cancelChooseRW := request(nh, "POST", "/api/disk/choose", "")
	if err := json.Unmarshal(cancelChooseRW.Body.Bytes(), &chooseBody); err != nil || !chooseBody.Cancelled {
		t.Fatalf("cancelled choose body = %+v, err=%v", chooseBody, err)
	}

	// Reveal boundary checks.
	if rw := request(nh, "POST", "/api/disk/reveal", `{"path":`+jsonQuote(filePath)+`}`); rw.Code != http.StatusNoContent {
		t.Fatalf("reveal valid file status = %d", rw.Code)
	}
	if len(nativeHost.revealed) != 1 || nativeHost.revealed[0] != filePath {
		t.Fatalf("revealed = %v, want [%s]", nativeHost.revealed, filePath)
	}
	if rw := request(nh, "POST", "/api/disk/reveal", `{"path":"relative.bin"}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("relative reveal status = %d, want 400", rw.Code)
	}
	if rw := request(nh, "POST", "/api/disk/reveal", `{"path":`+jsonQuote(outsideFile)+`}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("outside reveal status = %d, want 400", rw.Code)
	}
	if rw := request(nh, "POST", "/api/disk/reveal", `{"path":`+jsonQuote(filepath.Join(root, "missing.bin"))+`}`); rw.Code != http.StatusNotFound {
		t.Fatalf("missing reveal status = %d, want 404", rw.Code)
	}
}

func TestLocalizedDiskHTML(t *testing.T) {
	cases := []struct {
		name    string
		loc     Locale
		want    []string
		notWant []string
	}{
		{
			name:    "simplified chinese",
			loc:     LocaleZhCN,
			want:    []string{`<html lang="zh-CN">`, "磁盘大文件详情", "扫描目录", "选择目录…", "开始扫描", "大文件列表", "APFS"},
			notWant: []string{`<html lang="en">`, "Large Disk Files", "Choose Folder…"},
		},
		{
			name:    "english",
			loc:     LocaleEn,
			want:    []string{`<html lang="en">`, "Large Disk Files", "Scan Folder", "Choose Folder…", "Largest Files", "APFS"},
			notWant: []string{`<html lang="zh-CN">`, "磁盘大文件详情", "选择目录…"},
		},
		{
			name:    "unknown locale defaults to english",
			loc:     Locale("fr"),
			want:    []string{`<html lang="en">`, "Large Disk Files", "Largest Files"},
			notWant: []string{`<html lang="zh-CN">`, "磁盘大文件详情"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler(&testHost{}, fixedSnapshots{}, nil, nil, tc.loc)
			for _, path := range []string{"/disk", "/disk.html"} {
				rw := request(h, "GET", path, "")
				if rw.Code != http.StatusOK {
					t.Fatalf("%s %s: status %d", tc.name, path, rw.Code)
				}
				if ct := rw.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
					t.Fatalf("%s %s: Content-Type = %q", tc.name, path, ct)
				}
				body := rw.Body.String()
				for _, s := range tc.want {
					if !strings.Contains(body, s) {
						t.Errorf("%s %s: missing %q", tc.name, path, s)
					}
				}
				for _, s := range tc.notWant {
					if strings.Contains(body, s) {
						t.Errorf("%s %s: unexpected %q", tc.name, path, s)
					}
				}
			}
		})
	}
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
