//go:build darwin

package stats

import (
	"syscall"
	"testing"
)

func TestDiskCapacityUsesAvailableBlocks(t *testing.T) {
	// Free space can include blocks reserved for the system. Those blocks
	// must not inflate the available capacity shown to the user.
	disk := diskFromStatfs(syscall.Statfs_t{Bsize: 4096, Blocks: 1000, Bfree: 300, Bavail: 250})
	want := Disk{Total: 4096000, Used: 3072000, Available: 1024000}
	if disk != want || disk.Usage() != 75 {
		t.Fatalf("disk = %+v, usage = %v", disk, disk.Usage())
	}
}

func TestDiskCapacityBoundaryReadings(t *testing.T) {
	for _, test := range []struct {
		name string
		info syscall.Statfs_t
		want Disk
	}{
		{"unavailable", syscall.Statfs_t{}, Disk{}},
		{"no blocks", syscall.Statfs_t{Bsize: 4096}, Disk{}},
		{"full", syscall.Statfs_t{Bsize: 4096, Blocks: 1000}, Disk{Total: 4096000, Used: 4096000}},
		{"empty", syscall.Statfs_t{Bsize: 4096, Blocks: 1000, Bavail: 1000}, Disk{Total: 4096000, Available: 4096000}},
		{"excess available", syscall.Statfs_t{Bsize: 4096, Blocks: 1000, Bavail: 1001}, Disk{Total: 4096000, Available: 4096000}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := diskFromStatfs(test.info); got != test.want {
				t.Fatalf("disk = %+v, want %+v", got, test.want)
			}
		})
	}
	if (Disk{}).Usage() != 0 {
		t.Fatal("unavailable capacity produced a nonzero usage")
	}
}
