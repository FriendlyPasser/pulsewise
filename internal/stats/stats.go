// Package stats samples the machine's CPU, GPU and memory once a second
// and keeps a short history of each for the panel's sparklines.
package stats

import (
	"sync"
	"time"
)

// HistoryLen is how many samples a sparkline draws (the design's 17 points).
const HistoryLen = 17

type CPU struct {
	Usage   float64   `json:"usage"` // user + system, 0–100
	User    float64   `json:"user"`
	System  float64   `json:"system"`
	Idle    float64   `json:"idle"`
	History []float64 `json:"history"`
}

type GPU struct {
	Usage   float64   `json:"usage"` // -1 when the driver says nothing
	Model   string    `json:"model"`
	Cores   int       `json:"cores"`
	History []float64 `json:"history"`
}

// Memory is in bytes, split the way Activity Monitor splits it.
type Memory struct {
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	App        uint64 `json:"app"`
	Wired      uint64 `json:"wired"`
	Compressed uint64 `json:"compressed"`
	Swap       uint64 `json:"swap"`
	// History is the last HistoryLen samples, for the stacked area chart.
	History []MemPoint `json:"history"`
}

// MemPoint is one sample of Memory's three stacked layers, in bytes.
type MemPoint struct {
	App        uint64 `json:"app"`
	Wired      uint64 `json:"wired"`
	Compressed uint64 `json:"compressed"`
}

type Snapshot struct {
	CPU    CPU    `json:"cpu"`
	GPU    GPU    `json:"gpu"`
	Memory Memory `json:"memory"`
	At     int64  `json:"at"`
}

// Sampler reads the machine every Interval; Latest is safe from any goroutine.
type Sampler struct {
	Interval time.Duration

	mu      sync.RWMutex
	snap    Snapshot
	cpuHist []float64
	gpuHist []float64
	memHist []MemPoint
	prev    ticks
	subs    []func(Snapshot)
}

func NewSampler(interval time.Duration) *Sampler {
	s := &Sampler{Interval: interval}
	s.prev, _ = readTicks()
	s.sample()
	return s
}

// OnSample calls f after every sample (the tray icon redraws from it).
func (s *Sampler) OnSample(f func(Snapshot)) {
	s.mu.Lock()
	s.subs = append(s.subs, f)
	s.mu.Unlock()
}

func (s *Sampler) Run() {
	t := time.NewTicker(s.Interval)
	for range t.C {
		s.sample()
	}
}

func (s *Sampler) Latest() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

func (s *Sampler) sample() {
	cpu := CPU{Idle: 100}
	if now, ok := readTicks(); ok {
		du, ds, di := now.user-s.prev.user, now.sys-s.prev.sys, now.idle-s.prev.idle
		if total := float64(du + ds + di); total > 0 {
			cpu.User = float64(du) / total * 100
			cpu.System = float64(ds) / total * 100
			cpu.Idle = float64(di) / total * 100
		}
		s.prev = now
	}
	cpu.Usage = cpu.User + cpu.System
	gpu := readGPU()
	mem := readMemory()

	s.mu.Lock()
	s.cpuHist = push(s.cpuHist, cpu.Usage)
	s.gpuHist = push(s.gpuHist, max(gpu.Usage, 0))
	cpu.History = append([]float64(nil), s.cpuHist...)
	s.memHist = push(s.memHist, MemPoint{mem.App, mem.Wired, mem.Compressed})
	gpu.History = append([]float64(nil), s.gpuHist...)
	mem.History = append([]MemPoint(nil), s.memHist...)
	s.snap = Snapshot{CPU: cpu, GPU: gpu, Memory: mem, At: time.Now().UnixMilli()}
	snap, subs := s.snap, append([]func(Snapshot){}, s.subs...)
	s.mu.Unlock()

	for _, f := range subs {
		f(snap)
	}
}

// push appends v, dropping the oldest so h holds at most HistoryLen.
func push[T any](h []T, v T) []T {
	h = append(h, v)
	if len(h) > HistoryLen {
		h = h[len(h)-HistoryLen:]
	}
	return h
}
