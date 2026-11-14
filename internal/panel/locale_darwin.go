//go:build darwin && cgo

package panel

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static char *cfStringToUTF8(CFStringRef s) {
	if (s == NULL) {
		return NULL;
	}
	CFIndex len = CFStringGetLength(s);
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(len, kCFStringEncodingUTF8) + 1;
	char *buf = (char *)malloc((size_t)maxSize);
	if (buf == NULL) {
		return NULL;
	}
	if (!CFStringGetCString(s, buf, maxSize, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}
*/
import "C"
import "unsafe"

// SystemLocale returns the UI locale matched against macOS preferred languages.
func SystemLocale() Locale {
	langs := C.CFLocaleCopyPreferredLanguages()
	if langs == 0 {
		return envLocale()
	}
	defer C.CFRelease(C.CFTypeRef(langs))

	count := C.CFArrayGetCount(langs)
	if count <= 0 {
		return envLocale()
	}
	preferred := make([]string, 0, int(count))
	for i := C.CFIndex(0); i < count; i++ {
		item := C.CFArrayGetValueAtIndex(langs, i)
		if item == nil {
			continue
		}
		cstr := C.cfStringToUTF8(C.CFStringRef(item))
		if cstr == nil {
			continue
		}
		preferred = append(preferred, C.GoString(cstr))
		C.free(unsafe.Pointer(cstr))
	}
	if len(preferred) == 0 {
		return envLocale()
	}
	return MatchLocale(preferred)
}
