package stats

import (
	"context"
	"sync"
	"time"
)

// HistoryLen is how many samples a sparkline draws.
const HistoryLen = 17

// Sampler owns the latest snapshot and history. Run has one caller; Latest and
// Subscribe are safe from any goroutine and return independently owned slices.
type Sampler struct {
	interval time.Duration
	read     func() Snapshot

	mu   sync.RWMutex
	snap Snapshot
	subs map[chan Snapshot]struct{}
}

func NewSampler(interval time.Duration) *Sampler {
	return newSampler(interval, newSystemReader())
}

func newSampler(interval time.Duration, read func() Snapshot) *Sampler {
	s := &Sampler{interval: interval, read: read, subs: make(map[chan Snapshot]struct{})}
	s.sample()
	return s
}

// Subscribe immediately delivers the current snapshot, then keeps only the
// newest unread snapshot. A slow consumer never blocks sampling. Canceling ctx
// removes the subscription and closes its channel.
func (s *Sampler) Subscribe(ctx context.Context) <-chan Snapshot {
	ch := make(chan Snapshot, 1)
	s.mu.Lock()
	if ctx.Err() != nil {
		close(ch)
		s.mu.Unlock()
		return ch
	}
	s.subs[ch] = struct{}{}
	ch <- s.snap.clone()
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.subs, ch)
		close(ch)
		s.mu.Unlock()
	}()
	return ch
}

func (s *Sampler) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sample()
		}
	}
}

func (s *Sampler) Latest() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap.clone()
}

func (s *Sampler) sample() {
	snap := s.read()
	snap.At = time.Now().UnixMilli()
	s.mu.Lock()
	defer s.mu.Unlock()
	snap.CPU.History = push(s.snap.CPU.History, snap.CPU.Usage)
	snap.GPU.History = push(s.snap.GPU.History, max(snap.GPU.Usage, 0))
	snap.Memory.History = push(s.snap.Memory.History, MemPoint{
		App: snap.Memory.App, Wired: snap.Memory.Wired, Compressed: snap.Memory.Compressed,
	})
	s.snap = snap
	for ch := range s.subs {
		// All channel writes and closes happen under mu. If the consumer read
		// the old value meanwhile, the nonblocking drain leaves the slot free.
		select {
		case <-ch:
		default:
		}
		ch <- snap.clone()
	}
}

func push[T any](h []T, v T) []T {
	h = append(h, v)
	if len(h) > HistoryLen {
		h = h[len(h)-HistoryLen:]
	}
	return h
}
