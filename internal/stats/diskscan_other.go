//go:build !darwin

package stats

import "os"

func statDevice(os.FileInfo) (uint64, bool) {
	return 0, false
}

func statRegularFile(info os.FileInfo) (regularFileStat, bool) {
	sz := uint64(max(info.Size(), 0))
	return regularFileStat{
		Allocated: sz,
		Logical:   sz,
		Nlink:     1,
	}, true
}
