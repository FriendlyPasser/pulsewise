package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunStopsBeforeInitialCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	u := &Updater{ctx: ctx, state: Idle}
	done := make(chan struct{})
	go func() { defer close(done); u.Run() }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run ignored application shutdown")
	}
	u.Check()
	if state := u.Status().State; state != Idle {
		t.Fatalf("a check started after shutdown: %s", state)
	}
}

func TestApplicationShutdownCancelsFeedRequest(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer srv.Close()
	t.Setenv("PULSEWISE_UPDATE_FEED", srv.URL)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	u := &Updater{ctx: ctx, state: Idle, current: "0.1.0"}
	u.Check()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("the check did not reach the feed")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("the feed request outlived the application")
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		status := u.Status()
		if status.State == Failed {
			if !strings.Contains(status.Error, "context canceled") {
				t.Fatalf("error = %q", status.Error)
			}
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("the canceled check did not finish")
		case <-tick.C:
		}
	}
}
