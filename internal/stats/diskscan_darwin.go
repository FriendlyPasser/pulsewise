//go:build darwin

package stats

import (
	"os"
	"syscall"
)

func statDevice(info os.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, false
	}
	return uint64(st.Dev), true
}

func statRegularFile(info os.FileInfo) (regularFileStat, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return regularFileStat{}, false
	}
	return regularFileStat{
		Allocated: uint64(max(st.Blocks, 0)) * 512,
		Logical:   uint64(max(st.Size, 0)),
		Dev:       uint64(st.Dev),
		Ino:       st.Ino,
		Nlink:     uint64(max(st.Nlink, 0)),
	}, true
}
