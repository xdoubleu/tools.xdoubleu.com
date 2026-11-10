package main

import "tools.xdoubleu.com/selfupdate/appbundle"

// runningInAppBundle reports whether execPath is inside a .app bundle rather
// than a raw dev binary.
func runningInAppBundle(execPath string) bool {
	return appbundle.Path(execPath) != ""
}
