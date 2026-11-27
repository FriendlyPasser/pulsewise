//go:build darwin

package stats

/*
#include <errno.h>
#include <stdlib.h>
#include <time.h>
#include <sys/sysctl.h>
#include <sys/socket.h>
#include <net/if.h>
#include <net/if_mib.h>
#include <net/if_types.h>

static void ifmib_entry_fields(const struct ifmibdata *entry, uint8_t *if_type,
                               uint32_t *flags, uint64_t *rx, uint64_t *tx) {
	*if_type = (uint8_t)entry->ifmd_data.ifi_type;
	*flags = (uint32_t)entry->ifmd_flags;
	*rx = (uint64_t)entry->ifmd_data.ifi_ibytes;
	*tx = (uint64_t)entry->ifmd_data.ifi_obytes;
}

static int read_ifmib_all(struct ifmibdata **out, size_t *out_count) {
	*out = NULL;
	*out_count = 0;

	int mib[6] = { CTL_NET, PF_LINK, NETLINK_GENERIC, IFMIB_IFALLDATA, 0, IFDATA_GENERAL };
	for (int attempt = 0; attempt < 3; attempt++) {
		size_t len = 0;
		if (sysctl(mib, 6, NULL, &len, NULL, 0) < 0) {
			return -1;
		}
		if (len == 0) {
			return 0;
		}
		size_t cap = len + 4 * sizeof(struct ifmibdata);
		struct ifmibdata *buf = (struct ifmibdata *)malloc(cap);
		if (!buf) {
			return -1;
		}
		len = cap;
		if (sysctl(mib, 6, buf, &len, NULL, 0) == 0) {
			*out = buf;
			*out_count = len / sizeof(struct ifmibdata);
			return 0;
		}
		int err = errno;
		free(buf);
		if (err != ENOMEM) {
			return -1;
		}
	}
	return -1;
}
*/
import "C"

import (
	"bytes"
	"unsafe"
)

func continuousNow() uint64 {
	return uint64(C.clock_gettime_nsec_np(C.CLOCK_MONOTONIC_RAW))
}

func darwinNetworkHeaderConstants() (up, loopback, pointToPoint uint32, ether uint8) {
	return uint32(C.IFF_UP), uint32(C.IFF_LOOPBACK), uint32(C.IFF_POINTOPOINT), uint8(C.IFT_ETHER)
}

func readNetworkInterfaces() ([]networkInterface, bool) {
	var raw *C.struct_ifmibdata
	var count C.size_t
	if C.read_ifmib_all(&raw, &count) != 0 {
		return nil, false
	}
	if raw == nil {
		return nil, true
	}
	defer C.free(unsafe.Pointer(raw))
	if count == 0 {
		return nil, true
	}

	entries := unsafe.Slice(raw, int(count))
	ifaces := make([]networkInterface, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		nameBytes := unsafe.Slice((*byte)(unsafe.Pointer(&entry.ifmd_name[0])), C.IFNAMSIZ)
		if end := bytes.IndexByte(nameBytes, 0); end >= 0 {
			nameBytes = nameBytes[:end]
		}
		var ifType C.uint8_t
		var flags C.uint32_t
		var rx, tx C.uint64_t
		C.ifmib_entry_fields(entry, &ifType, &flags, &rx, &tx)
		ifaces = append(ifaces, networkInterface{
			name:   string(nameBytes),
			ifType: uint8(ifType),
			flags:  uint32(flags),
			rx:     uint64(rx),
			tx:     uint64(tx),
		})
	}
	return ifaces, true
}
