//go:build !darwin

package main

import "tools.xdoubleu.com/kobo-gateway/internal/kobogateway"

// runUI blocks until stop closes; the menu bar needs AppKit (macOS only).
func runUI(
	_ string,
	stop <-chan struct{},
	_ <-chan kobogateway.KoboEvent,
	_, _ string,
	_ bool,
) {
	<-stop
}
