package main

import "tools.xdoubleu.com/kobo-gateway/internal/kobogateway"

// runningInAppBundle reports whether execPath points at a binary launched
// from inside a real .app bundle (e.g.
// /Applications/KoboGateway.app/Contents/MacOS/kobo-gateway), as opposed to
// a raw dev binary (e.g. ./bin/kobo-gateway-darwin-arm64). UNUserNotificationCenter
// requires bundleProxyForCurrentProcess to be non-nil, which is only true
// inside a bundle — calling it from a raw binary throws, so callers must
// check this first and skip notifications otherwise.
func runningInAppBundle(execPath string) bool {
	return kobogateway.AppBundlePath(execPath) != ""
}
