//go:build darwin && cgo

package gui

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c
#cgo LDFLAGS: -framework ServiceManagement -framework Foundation

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <stdlib.h>
#include <string.h>

enum {
	pulsewiseLoginDisabled = 0,
	pulsewiseLoginEnabled = 1,
	pulsewiseLoginRequiresApproval = 2,
	pulsewiseLoginUnavailable = 3,
};

enum {
	pulsewiseSetOK = 0,
	pulsewiseSetErr = -1,
	pulsewiseSetUnsupported = -2,
	pulsewiseSetRequiresApproval = -3,
};

static int pulsewiseLoginItemAvailable(void) {
	if (@available(macOS 13.0, *)) {
		return 1;
	}
	return 0;
}

static int pulsewiseLoginItemStatus(void) {
	if (@available(macOS 13.0, *)) {
		@autoreleasepool {
			switch (SMAppService.mainAppService.status) {
			case SMAppServiceStatusNotRegistered:
				return pulsewiseLoginDisabled;
			case SMAppServiceStatusEnabled:
				return pulsewiseLoginEnabled;
			case SMAppServiceStatusRequiresApproval:
				return pulsewiseLoginRequiresApproval;
			case SMAppServiceStatusNotFound:
			default:
				return pulsewiseLoginUnavailable;
			}
		}
	}
	return pulsewiseLoginUnavailable;
}

static void pulsewiseCopyNSError(NSError *err, char **errOut) {
	if (err == nil || errOut == NULL) {
		return;
	}
	const char *desc = err.localizedDescription.UTF8String;
	if (desc != NULL) {
		*errOut = strdup(desc);
	}
}

static int pulsewiseSetLoginItemEnabled(int enabled, char **errOut) {
	if (@available(macOS 13.0, *)) {
		@autoreleasepool {
			SMAppService *service = SMAppService.mainAppService;
			SMAppServiceStatus current = service.status;
			NSError *err = nil;
			if (enabled) {
				if (current == SMAppServiceStatusEnabled) {
					return pulsewiseSetOK;
				}
				if (current == SMAppServiceStatusRequiresApproval) {
					return pulsewiseSetRequiresApproval;
				}
				if ([service registerAndReturnError:&err]) {
					return pulsewiseSetOK;
				}
				if (err != nil) {
					if (err.code == kSMErrorAlreadyRegistered) {
						return pulsewiseSetOK;
					}
					if (err.code == kSMErrorLaunchDeniedByUser) {
						return pulsewiseSetRequiresApproval;
					}
					pulsewiseCopyNSError(err, errOut);
				}
				return pulsewiseSetErr;
			}
			if (current == SMAppServiceStatusNotRegistered || current == SMAppServiceStatusNotFound) {
				return pulsewiseSetOK;
			}
			if ([service unregisterAndReturnError:&err]) {
				return pulsewiseSetOK;
			}
			if (err != nil) {
				if (err.code == kSMErrorJobNotFound) {
					return pulsewiseSetOK;
				}
				pulsewiseCopyNSError(err, errOut);
			}
			return pulsewiseSetErr;
		}
	}
	return pulsewiseSetUnsupported;
}

static int pulsewiseOpenLoginItemsSettings(void) {
	if (@available(macOS 13.0, *)) {
		@autoreleasepool {
			[SMAppService openSystemSettingsLoginItems];
			return pulsewiseSetOK;
		}
	}
	return pulsewiseSetUnsupported;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func newLoginItemDriver() loginItemDriver {
	return darwinLoginItemDriver{}
}

type darwinLoginItemDriver struct{}

func (darwinLoginItemDriver) available() bool {
	return C.pulsewiseLoginItemAvailable() != 0
}

func (darwinLoginItemDriver) status() (launchStatus, error) {
	if C.pulsewiseLoginItemAvailable() == 0 {
		return launchUnavailable, errLaunchUnsupported
	}
	switch C.pulsewiseLoginItemStatus() {
	case C.pulsewiseLoginDisabled:
		return launchDisabled, nil
	case C.pulsewiseLoginEnabled:
		return launchEnabled, nil
	case C.pulsewiseLoginRequiresApproval:
		return launchRequiresApproval, nil
	default:
		return launchUnavailable, nil
	}
}

func (darwinLoginItemDriver) setEnabled(enabled bool) error {
	var flag C.int
	if enabled {
		flag = 1
	}
	var cErr *C.char
	rc := C.pulsewiseSetLoginItemEnabled(flag, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
	}
	switch rc {
	case C.pulsewiseSetOK:
		return nil
	case C.pulsewiseSetRequiresApproval:
		return errLaunchRequiresApproval
	case C.pulsewiseSetUnsupported:
		return errLaunchUnsupported
	default:
		if cErr != nil {
			return errors.New(C.GoString(cErr))
		}
		return errors.New("ServiceManagement operation failed")
	}
}

func (darwinLoginItemDriver) openSystemSettings() error {
	if C.pulsewiseOpenLoginItemsSettings() == C.pulsewiseSetUnsupported {
		return errLaunchUnsupported
	}
	return nil
}
