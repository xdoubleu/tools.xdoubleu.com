//go:build !darwin

package main

// runUI is a no-op on non-darwin platforms (the menu bar needs AppKit,
// which only exists on macOS); it just blocks until stop closes, keeping
// the compile check green on Linux CI and local dev.
func runUI(_ string, stop <-chan struct{}) {
	<-stop
}
