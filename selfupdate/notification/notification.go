// Package notification posts macOS notifications for the menu-bar helpers.
// UNUserNotificationCenter throws outside a .app bundle, so every call is a no-op
// there, and on platforms without cgo and AppKit.
package notification

import (
	"os"

	"tools.xdoubleu.com/selfupdate/appbundle"
)

// Available reports whether notifications can be shown from this process.
func Available() bool {
	executable, err := os.Executable()
	return err == nil && appbundle.Path(executable) != "" && supported
}

// RequestAuth asks macOS for permission to notify, once per process. Call it
// from the app's launch.
func RequestAuth() {
	if Available() {
		requestAuth()
	}
}

// Post shows a best-effort notification, safe from any goroutine.
func Post(title, body string) {
	if Available() {
		post(title, body)
	}
}
