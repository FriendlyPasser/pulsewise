//go:build darwin

package stats

import "syscall"

func readDisk() Disk {
	var info syscall.Statfs_t
	// The sealed root volume only reports its own files as used on APFS.
	// Read the writable data volume and count all unavailable capacity instead.
	err := syscall.Statfs("/System/Volumes/Data", &info)
	if err == syscall.ENOENT {
		err = syscall.Statfs("/", &info)
	}
	if err != nil {
		return Disk{}
	}
	return diskFromStatfs(info)
}

func diskFromStatfs(info syscall.Statfs_t) Disk {
	if info.Bsize == 0 || info.Blocks == 0 {
		return Disk{}
	}
	total := info.Blocks * uint64(info.Bsize)
	available := min(info.Bavail, info.Blocks) * uint64(info.Bsize)
	return Disk{Total: total, Used: total - available, Available: available}
}
