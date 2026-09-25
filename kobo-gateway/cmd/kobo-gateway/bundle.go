package main

import "tools.xdoubleu.com/kobo-gateway/internal/kobogateway"

// runningInAppBundle reports whether execPath is inside a .app bundle rather
// than a raw dev binary. UNUserNotificationCenter throws outside a bundle, so
// check this before notifying.
func runningInAppBundle(execPath string) bool {
	return kobogateway.AppBundlePath(execPath) != ""
}
