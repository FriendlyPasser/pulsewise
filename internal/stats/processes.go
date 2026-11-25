package stats

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

type rawProcess struct {
	pid    int
	ppid   int
	rpid   int
	name   string
	path   string
	memory uint64
}

// ReadMemoryDetail captures current-user process memory footprints grouped by
// owning application alongside system memory totals.
func ReadMemoryDetail() MemoryDetail {
	d := groupProcesses(readProcesses())
	m := readMemory()
	if m.Total > 0 {
		m.History = []MemPoint{{App: m.App, Wired: m.Wired, Compressed: m.Compressed}}
	}
	d.Memory = m
	d.At = time.Now().UnixMilli()
	return d
}

func groupProcesses(procs []rawProcess) MemoryDetail {
	detail := MemoryDetail{
		Apps:         []AppMemory{},
		Unattributed: []ProcessMemory{},
	}
	byPID := make(map[int]rawProcess, len(procs))
	directApp := make(map[int]string, len(procs))
	filtered := make([]rawProcess, 0, len(procs))
	for _, p := range procs {
		if p.pid <= 0 || p.memory == 0 {
			continue
		}
		filtered = append(filtered, p)
		byPID[p.pid] = p
		if app := outermostAppName(p.path); app != "" {
			directApp[p.pid] = app
		}
	}

	appIndex := make(map[string]int)
	for _, p := range filtered {
		proc := ProcessMemory{
			PID:    p.pid,
			Name:   executableName(p.path, p.name),
			Memory: p.memory,
		}
		app := resolveApp(p.pid, byPID, directApp)
		if app == "" {
			detail.Unattributed = append(detail.Unattributed, proc)
			detail.UnattributedMemory += proc.Memory
			continue
		}
		idx, ok := appIndex[app]
		if !ok {
			idx = len(detail.Apps)
			appIndex[app] = idx
			detail.Apps = append(detail.Apps, AppMemory{Name: app})
		}
		detail.Apps[idx].Memory += proc.Memory
		detail.Apps[idx].Processes = append(detail.Apps[idx].Processes, proc)
	}

	for i := range detail.Apps {
		slices.SortFunc(detail.Apps[i].Processes, compareProcessMemory)
	}
	slices.SortFunc(detail.Apps, func(a, b AppMemory) int {
		return cmp.Or(cmp.Compare(b.Memory, a.Memory), cmp.Compare(a.Name, b.Name))
	})
	slices.SortFunc(detail.Unattributed, compareProcessMemory)
	return detail
}

func compareProcessMemory(a, b ProcessMemory) int {
	return cmp.Or(cmp.Compare(b.Memory, a.Memory), cmp.Compare(a.PID, b.PID))
}

func resolveApp(pid int, byPID map[int]rawProcess, directApp map[int]string) string {
	seen := make(map[int]struct{}, 8)
	cur := pid
	for range 32 {
		if cur <= 1 {
			return ""
		}
		if _, ok := seen[cur]; ok {
			return ""
		}
		seen[cur] = struct{}{}
		if app := directApp[cur]; app != "" {
			return app
		}
		p, ok := byPID[cur]
		if !ok {
			return ""
		}
		if p.rpid > 1 && p.rpid != cur {
			if app := directApp[p.rpid]; app != "" {
				return app
			}
		}
		if p.ppid > 1 && p.ppid != cur {
			cur = p.ppid
			continue
		}
		if p.rpid > 1 && p.rpid != cur {
			cur = p.rpid
			continue
		}
		return ""
	}
	return ""
}

// outermostAppName extracts the outermost .app or .app.bundle directory name
// so nested helper bundles and Chromium code_sign_clone bundles collapse into
// their owning application.
func outermostAppName(path string) string {
	bestIdx := -1
	bestEnd := 0
	for _, marker := range []struct {
		needle string
		suffix string
	}{
		{".app/Contents/", ".app"},
		{".app.bundle/Contents/", ".app.bundle"},
	} {
		if idx := strings.Index(path, marker.needle); idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			bestEnd = idx + len(marker.suffix)
		}
	}
	if bestIdx < 0 {
		return ""
	}
	bundle := path[:bestEnd]
	if slash := strings.LastIndexByte(bundle, '/'); slash >= 0 {
		bundle = bundle[slash+1:]
	}
	bundle = strings.TrimSuffix(bundle, ".app.bundle")
	return strings.TrimSuffix(bundle, ".app")
}

func executableName(path, fallback string) string {
	if path != "" {
		base := path
		if slash := strings.LastIndexByte(base, '/'); slash >= 0 {
			base = base[slash+1:]
		}
		if base != "" {
			return base
		}
	}
	return fallback
}
