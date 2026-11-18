//go:build darwin

package stats

/*
#include <libproc.h>
#include <stdlib.h>
#include <string.h>
#include <sys/proc_info.h>

// Weak-import Apple's private responsibility API so XPC services launched by
// launchd resolve back to the app responsible for spawning them when present.
extern pid_t responsibility_get_pid_responsible_for_pid(pid_t pid) __attribute__((weak_import));

static int read_proc_sample(pid_t pid, pid_t *ppid, pid_t *rpid,
                            unsigned long long *footprint,
                            char *name, int name_len,
                            char *path, int path_len) {
	struct rusage_info_v4 ru;
	if (proc_pid_rusage(pid, RUSAGE_INFO_V4, (rusage_info_t *)&ru) != 0 || ru.ri_phys_footprint == 0) {
		return -1;
	}
	*footprint = ru.ri_phys_footprint;
	*ppid = 0;
	*rpid = 0;
	if (name_len > 0) name[0] = 0;
	if (path_len > 0) path[0] = 0;

	struct proc_bsdshortinfo sbsd;
	if (proc_pidinfo(pid, PROC_PIDT_SHORTBSDINFO, 0, &sbsd, sizeof(sbsd)) == (int)sizeof(sbsd)) {
		*ppid = (pid_t)sbsd.pbsi_ppid;
		if (name_len > 0) {
			strncpy(name, sbsd.pbsi_comm, (size_t)(name_len - 1));
			name[name_len - 1] = 0;
		}
	}
	if (responsibility_get_pid_responsible_for_pid != NULL) {
		pid_t resp = responsibility_get_pid_responsible_for_pid(pid);
		if (resp > 0) {
			*rpid = resp;
		}
	}
	char procname[256];
	procname[0] = 0;
	if (proc_name(pid, procname, sizeof(procname)) > 0 && procname[0] != 0 && name_len > 0) {
		strncpy(name, procname, (size_t)(name_len - 1));
		name[name_len - 1] = 0;
	}
	if (path_len > 0 && proc_pidpath(pid, path, (uint32_t)path_len) <= 0) {
		path[0] = 0;
	}
	return 0;
}
*/
import "C"

import (
	"bytes"
	"unsafe"
)

func readProcesses() []rawProcess {
	n := int(C.proc_listallpids(nil, 0))
	if n <= 0 {
		return nil
	}
	capPIDs := max(n*2, 1024)
	pids := make([]C.pid_t, capPIDs)
	// proc_listallpids takes buffer size in bytes and returns a PID count.
	count := int(C.proc_listallpids(unsafe.Pointer(&pids[0]), C.int(capPIDs*int(unsafe.Sizeof(C.pid_t(0))))))
	if count <= 0 {
		return nil
	}
	if count > capPIDs {
		count = capPIDs
	}

	nameBuf := make([]byte, 256)
	pathBuf := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	out := make([]rawProcess, 0, count)
	for _, pid := range pids[:count] {
		if pid <= 0 {
			continue
		}
		var ppid, rpid C.pid_t
		var footprint C.ulonglong
		if C.read_proc_sample(
			pid, &ppid, &rpid, &footprint,
			(*C.char)(unsafe.Pointer(&nameBuf[0])), C.int(len(nameBuf)),
			(*C.char)(unsafe.Pointer(&pathBuf[0])), C.int(len(pathBuf)),
		) != 0 {
			continue
		}
		out = append(out, rawProcess{
			pid:    int(pid),
			ppid:   int(ppid),
			rpid:   int(rpid),
			name:   cString(nameBuf),
			path:   cString(pathBuf),
			memory: uint64(footprint),
		})
	}
	return out
}

func cString(buf []byte) string {
	if i := bytes.IndexByte(buf, 0); i >= 0 {
		return string(buf[:i])
	}
	return string(buf)
}
