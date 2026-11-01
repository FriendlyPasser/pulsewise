package stats

type ticks struct{ user, sys, idle uint64 }

// The system reader owns the previous counters. The sampler only sees readings.
func newSystemReader() func() Snapshot {
	prev, _ := readTicks()
	return func() Snapshot {
		cpu := CPU{Idle: 100}
		if now, ok := readTicks(); ok {
			cpu = cpuBetween(prev, now)
			prev = now
		}
		return Snapshot{CPU: cpu, GPU: readGPU(), Memory: readMemory()}
	}
}

func cpuBetween(prev, now ticks) CPU {
	cpu := CPU{Idle: 100}
	du, ds, di := now.user-prev.user, now.sys-prev.sys, now.idle-prev.idle
	if total := float64(du + ds + di); total > 0 {
		cpu.User = float64(du) / total * 100
		cpu.System = float64(ds) / total * 100
		cpu.Idle = float64(di) / total * 100
	}
	cpu.Usage = cpu.User + cpu.System
	return cpu
}
