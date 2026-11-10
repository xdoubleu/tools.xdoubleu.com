//go:build !darwin || !cgo

package main

import (
	"context"

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
)

// runUI blocks until stop closes; the menu bar needs cgo and AppKit.
func runUI(
	_ string,
	stop <-chan struct{},
	_ <-chan kobogateway.KoboEvent,
	_, _ string,
	_ bool,
	_ func(context.Context) (bool, error),
) {
	<-stop
}
