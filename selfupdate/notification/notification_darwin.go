//go:build darwin && cgo

package notification

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>
#include "notification_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

const supported = true

var authOnce sync.Once //nolint:gochecknoglobals // one-shot guard per process

func requestAuth() {
	authOnce.Do(func() { C.snRequestAuth() })
}

func post(title, body string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))
	C.snPost(cTitle, cBody)
}
