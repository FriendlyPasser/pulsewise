package panel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FriendlyPasser/pulsewise/internal/stats"
	"github.com/FriendlyPasser/pulsewise/internal/update"
)

type fixedSnapshots struct{ snapshot stats.Snapshot }

func (s fixedSnapshots) Latest() stats.Snapshot { return s.snapshot }

type testHost struct {
	height int
	quit   chan struct{}
}

func (h *testHost) FitPanel(height int) { h.height = height }
func (h *testHost) Quit()               { h.quit <- struct{}{} }

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
	}
	h := Handler(&testHost{}, fixedSnapshots{want}, nil)
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

func TestFitValidatesAtHTTPBoundary(t *testing.T) {
	for _, body := range []string{`{}`, `{"height":0}`, `{"height":-1}`, `{"height":"440"}`, `broken`} {
		t.Run(body, func(t *testing.T) {
			host := &testHost{}
			rw := request(Handler(host, fixedSnapshots{}, nil), "POST", "/api/fit", body)
			if rw.Code != http.StatusBadRequest || host.height != 0 {
				t.Fatalf("response %d, height %d", rw.Code, host.height)
			}
		})
	}
	host := &testHost{}
	rw := request(Handler(host, fixedSnapshots{}, nil), "POST", "/api/fit", `{"height":440}`)
	if rw.Code != http.StatusNoContent || host.height != 440 {
		t.Fatalf("response %d, height %d", rw.Code, host.height)
	}
}

func TestEmbeddedAssets(t *testing.T) {
	h := Handler(&testHost{}, fixedSnapshots{}, nil)
	for path, text := range map[string]string{"/": "cpuValue", "/app.js": "/api/state", "/app.css": "--panel"} {
		rw := request(h, "GET", path, "")
		if rw.Code != http.StatusOK || !strings.Contains(rw.Body.String(), text) {
			t.Fatalf("asset %s: status %d", path, rw.Code)
		}
	}
}

func TestBrowserHasNoUpdateCommands(t *testing.T) {
	h := Handler(&testHost{}, fixedSnapshots{}, nil)
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
	h := Handler(host, fixedSnapshots{}, u)
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
	h := Handler(host, fixedSnapshots{}, u)
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
	rw := request(Handler(host, fixedSnapshots{}, nil), "POST", "/api/quit", "")
	if rw.Code != http.StatusNoContent {
		t.Fatalf("status %d", rw.Code)
	}
	select {
	case <-host.quit:
	case <-time.After(time.Second):
		t.Fatal("quit was not delivered")
	}
}
