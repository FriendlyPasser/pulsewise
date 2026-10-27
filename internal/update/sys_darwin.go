package update

import (
	"os/exec"
	"syscall"
)

// errCrossDevice is rename's error for a stage on another volume.
const errCrossDevice = syscall.EXDEV

// readOnly reports whether path is on a read-only volume, as a mounted
// disk image is.
func readOnly(path string) bool {
	var st syscall.Statfs_t
	return syscall.Statfs(path, &st) == nil && st.Flags&0x1 != 0 // MNT_RDONLY
}

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
