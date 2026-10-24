//go:build darwin

package stats

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <sys/sysctl.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// cpu_ticks fills the machine's CPU ticks since boot, summed over all cores.
static int cpu_ticks(unsigned long long *user, unsigned long long *sys,
                     unsigned long long *idle, unsigned long long *nice) {
	host_cpu_load_info_data_t info;
	mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
	if (host_statistics(mach_host_self(), HOST_CPU_LOAD_INFO, (host_info_t)&info, &count) != KERN_SUCCESS)
		return -1;
	*user = info.cpu_ticks[CPU_STATE_USER];
	*sys = info.cpu_ticks[CPU_STATE_SYSTEM];
	*idle = info.cpu_ticks[CPU_STATE_IDLE];
	*nice = info.cpu_ticks[CPU_STATE_NICE];
	return 0;
}

// vm_pages reads the page counts Activity Monitor's memory tab is made of.
static int vm_pages(unsigned long long *internal, unsigned long long *purgeable,
                    unsigned long long *wired, unsigned long long *compressed,
                    unsigned long long *pagesize) {
	vm_statistics64_data_t vm;
	mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
	if (host_statistics64(mach_host_self(), HOST_VM_INFO64, (host_info64_t)&vm, &count) != KERN_SUCCESS)
		return -1;
	vm_size_t ps;
	host_page_size(mach_host_self(), &ps);
	*internal = vm.internal_page_count;
	*purgeable = vm.purgeable_count;
	*wired = vm.wire_count;
	*compressed = vm.compressor_page_count;
	*pagesize = ps;
	return 0;
}

static unsigned long long mem_total(void) {
	unsigned long long v = 0;
	size_t n = sizeof(v);
	sysctlbyname("hw.memsize", &v, &n, NULL, 0);
	return v;
}

static unsigned long long swap_used(void) {
	struct xsw_usage u;
	size_t n = sizeof(u);
	if (sysctlbyname("vm.swapusage", &u, &n, NULL, 0) != 0)
		return 0;
	return u.xsu_used;
}

static long long num_of(CFDictionaryRef d, const char *key) {
	CFStringRef k = CFStringCreateWithCString(NULL, key, kCFStringEncodingUTF8);
	CFTypeRef v = CFDictionaryGetValue(d, k);
	CFRelease(k);
	long long out = -1;
	if (v && CFGetTypeID(v) == CFNumberGetTypeID())
		CFNumberGetValue((CFNumberRef)v, kCFNumberLongLongType, &out);
	return out;
}

// gpu_info reads the first IOAccelerator: its utilization (from the
// driver's PerformanceStatistics, no root needed), name and core count.
static int gpu_info(int *util, int *cores, char *model, int modelLen) {
	io_iterator_t it;
	if (IOServiceGetMatchingServices(kIOMainPortDefault, IOServiceMatching("IOAccelerator"), &it) != KERN_SUCCESS)
		return -1;
	int found = -1;
	io_object_t svc;
	while ((svc = IOIteratorNext(it)) != 0) {
		CFMutableDictionaryRef props = NULL;
		if (IORegistryEntryCreateCFProperties(svc, &props, kCFAllocatorDefault, 0) == KERN_SUCCESS && props) {
			CFTypeRef perf = CFDictionaryGetValue(props, CFSTR("PerformanceStatistics"));
			if (perf && CFGetTypeID(perf) == CFDictionaryGetTypeID()) {
				*util = (int)num_of((CFDictionaryRef)perf, "Device Utilization %");
				*cores = (int)num_of(props, "gpu-core-count");
				CFTypeRef m = CFDictionaryGetValue(props, CFSTR("model"));
				if (m && CFGetTypeID(m) == CFStringGetTypeID())
					CFStringGetCString((CFStringRef)m, model, modelLen, kCFStringEncodingUTF8);
				else if (m && CFGetTypeID(m) == CFDataGetTypeID()) {
					CFIndex n = CFDataGetLength((CFDataRef)m);
					if (n >= modelLen) n = modelLen - 1;
					CFDataGetBytes((CFDataRef)m, CFRangeMake(0, n), (UInt8 *)model);
					model[n] = 0;
				}
				found = 0;
			}
			CFRelease(props);
		}
		IOObjectRelease(svc);
		if (found == 0) break;
	}
	IOObjectRelease(it);
	return found;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

type ticks struct{ user, sys, idle uint64 }

func readTicks() (ticks, bool) {
	var u, s, i, n C.ulonglong
	if C.cpu_ticks(&u, &s, &i, &n) != 0 {
		return ticks{}, false
	}
	return ticks{user: uint64(u + n), sys: uint64(s), idle: uint64(i)}, true
}

func readMemory() Memory {
	var internal, purgeable, wired, compressed, ps C.ulonglong
	m := Memory{Total: uint64(C.mem_total())}
	if C.vm_pages(&internal, &purgeable, &wired, &compressed, &ps) == 0 {
		app := uint64(internal) - min(uint64(internal), uint64(purgeable))
		m.App = app * uint64(ps)
		m.Wired = uint64(wired) * uint64(ps)
		m.Compressed = uint64(compressed) * uint64(ps)
	}
	m.Used = m.App + m.Wired + m.Compressed
	m.Swap = uint64(C.swap_used())
	return m
}

func readGPU() GPU {
	var util, cores C.int
	buf := make([]byte, 128)
	g := GPU{Usage: -1}
	if C.gpu_info(&util, &cores, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf))) == 0 {
		g.Usage = float64(util)
		g.Cores = int(cores)
		g.Model = strings.TrimRight(string(buf), "\x00")
	}
	return g
}
