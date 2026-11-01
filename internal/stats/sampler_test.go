package stats

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func countingSampler(interval time.Duration) *Sampler {
	n := 0.0
	return newSampler(interval, func() Snapshot {
		n++
		return Snapshot{
			CPU: CPU{Usage: n}, GPU: GPU{Usage: -1},
			Memory: Memory{Total: 1024, App: uint64(n), Wired: 2, Compressed: 3},
		}
	})
}

func TestCPUUsesCounterDifferences(t *testing.T) {
	cpu := cpuBetween(ticks{100, 200, 300}, ticks{120, 210, 370})
	if cpu.User != 20 || cpu.System != 10 || cpu.Idle != 70 || cpu.Usage != 30 {
		t.Fatalf("CPU = %+v", cpu)
	}
	if idle := cpuBetween(ticks{}, ticks{}); idle.Usage != 0 || idle.Idle != 100 {
		t.Fatalf("no ticks = %+v", idle)
	}
}

func TestHistoryRetainsNewestSamples(t *testing.T) {
	s := countingSampler(time.Hour)
	for range HistoryLen + 4 {
		s.sample()
	}
	snap := s.Latest()
	if len(snap.CPU.History) != HistoryLen || len(snap.GPU.History) != HistoryLen || len(snap.Memory.History) != HistoryLen {
		t.Fatalf("history lengths = %d, %d, %d", len(snap.CPU.History), len(snap.GPU.History), len(snap.Memory.History))
	}
	for i := range HistoryLen {
		want := float64(i + 6)
		if snap.CPU.History[i] != want || snap.Memory.History[i].App != uint64(want) {
			t.Fatalf("point %d: CPU %v, memory %+v", i, snap.CPU.History[i], snap.Memory.History[i])
		}
		if snap.GPU.History[i] != 0 {
			t.Fatalf("unavailable GPU history = %v", snap.GPU.History[i])
		}
	}
	if snap.GPU.Usage != -1 || snap.At == 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestSnapshotsDoNotShareHistory(t *testing.T) {
	s := countingSampler(time.Hour)
	first := s.Subscribe(t.Context())
	second := s.Subscribe(t.Context())
	a, b := <-first, <-second
	latest := s.Latest()
	a.CPU.History[0] = 999
	a.GPU.History[0] = 999
	a.Memory.History[0].App = 999
	latest.CPU.History[0] = 888
	latest.GPU.History[0] = 888
	latest.Memory.History[0].App = 888
	if !reflect.DeepEqual(b, s.Latest()) {
		t.Fatal("a reader changed the cache or another subscription")
	}
	for range 2 * HistoryLen {
		s.sample()
	}
	if b.CPU.History[0] != 1 || b.GPU.History[0] != 0 || b.Memory.History[0].App != 1 {
		t.Fatal("later samples changed a retained snapshot")
	}
}

func TestSlowSubscriberGetsLatestWithoutBlockingSampling(t *testing.T) {
	s := countingSampler(time.Hour)
	ch := s.Subscribe(t.Context()) // Leave the initial value unread.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			s.sample()
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("an unread subscription blocked sampling")
	}
	if snap := <-ch; snap.CPU.Usage != 101 || len(snap.CPU.History) != HistoryLen {
		t.Fatalf("latest snapshot = %+v", snap)
	}
}

func TestRunPublishesAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := countingSampler(time.Millisecond)
	ch := s.Subscribe(ctx)
	<-ch
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	select {
	case snap := <-ch:
		if snap.CPU.Usage < 2 {
			t.Fatalf("next sample = %+v", snap)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not publish a sample")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run ignored cancellation")
	}
	closed := make(chan struct{})
	go func() {
		for range ch {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("subscription did not close")
	}
}

func TestConcurrentReadersOwnTheirSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := countingSampler(time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for range 100 {
				snap := s.Latest()
				snap.CPU.History[0] = -100
				snap.Memory.History[0].App = 9999
			}
		})
	}
	for range 20 {
		subCtx, stop := context.WithCancel(ctx)
		ch := s.Subscribe(subCtx)
		snap := <-ch
		snap.GPU.History[0] = -100
		stop()
	}
	readers.Wait()
	cancel()
	<-done
	if snap := s.Latest(); snap.CPU.History[0] < 0 || snap.GPU.History[0] < 0 || snap.Memory.History[0].App == 9999 {
		t.Fatal("a reader's mutations leaked into the sampler")
	}
	if _, ok := <-s.Subscribe(ctx); ok {
		t.Fatal("a canceled context created an active subscription")
	}
}
