//go:build !darwin

package main

import "tools.xdoubleu.com/gateway/internal/kobogateway"

// runUI is a no-op on non-darwin platforms (the menu bar needs AppKit,
// which only exists on macOS); it just blocks until stop closes, keeping
// the compile check green on Linux CI and local dev.
func runUI(
	_ string,
	stop <-chan struct{},
	_ <-chan kobogateway.KoboEvent,
	_, _ string,
) {
	<-stop
}
