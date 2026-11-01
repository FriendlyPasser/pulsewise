// Package stats samples system load and publishes snapshots with a short history.
package stats

import "slices"

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
	Total      uint64     `json:"total"`
	Used       uint64     `json:"used"`
	App        uint64     `json:"app"`
	Wired      uint64     `json:"wired"`
	Compressed uint64     `json:"compressed"`
	Swap       uint64     `json:"swap"`
	History    []MemPoint `json:"history"`
}

func (m Memory) Usage() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.Used) / float64(m.Total) * 100
}

// MemPoint is one sample of Memory's three stacked layers, in bytes.
type MemPoint struct {
	App        uint64 `json:"app"`
	Wired      uint64 `json:"wired"`
	Compressed uint64 `json:"compressed"`
}

// Disk is the startup data volume's capacity in bytes. Total is zero when
// unavailable. Used includes space unavailable to the current user.
type Disk struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
}

func (d Disk) Usage() float64 {
	if d.Total == 0 {
		return 0
	}
	return float64(d.Used) / float64(d.Total) * 100
}

// Snapshot is the shared read model for the tray and panel.
type Snapshot struct {
	CPU    CPU    `json:"cpu"`
	GPU    GPU    `json:"gpu"`
	Memory Memory `json:"memory"`
	Disk   Disk   `json:"disk"`
	At     int64  `json:"at"`
}

func (s Snapshot) clone() Snapshot {
	s.CPU.History = slices.Clone(s.CPU.History)
	s.GPU.History = slices.Clone(s.GPU.History)
	s.Memory.History = slices.Clone(s.Memory.History)
	return s
}
