package update

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type feedTransport func(*http.Request) (*http.Response, error)

func (f feedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAutomaticChecksEveryMinute(t *testing.T) {
	var checks atomic.Int32
	previousClient := client
	client = &http.Client{Transport: feedTransport(func(*http.Request) (*http.Response, error) {
		checks.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"version":"0.1.0"}`)),
		}, nil
	})}
	t.Cleanup(func() { client = previousClient })

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		u := &Updater{ctx: ctx, state: Idle, current: "0.1.0"}
		go u.Run()
		synctest.Wait()

		assertChecks := func(want int32) {
			t.Helper()
			synctest.Wait()
			if got := checks.Load(); got != want {
				t.Fatalf("feed checks = %d, want %d", got, want)
			}
		}
		time.Sleep(5*time.Second - time.Nanosecond)
		assertChecks(0)
		time.Sleep(time.Nanosecond)
		assertChecks(1)

		// A manual check must not suppress the next automatic minute tick.
		time.Sleep(30 * time.Second)
		u.Check()
		assertChecks(2)
		time.Sleep(30*time.Second - time.Nanosecond)
		assertChecks(2)
		time.Sleep(time.Nanosecond)
		assertChecks(3)
		time.Sleep(time.Minute)
		assertChecks(4)

		cancel()
		synctest.Wait()
		time.Sleep(time.Minute)
		assertChecks(4)
	})
}

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
