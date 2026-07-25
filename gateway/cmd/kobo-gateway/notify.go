// Command kobo-gateway — notify is the seam between the platform-agnostic
// server (internal/kobogateway) and the menu bar's toast notifications, so
// self-update lifecycle events (#456) are visible even though
// internal/kobogateway has no AppKit dependency.
package main

// notify shows a best-effort menu-bar notification. Overridden by
// menubar_darwin.go's init to the real UNUserNotificationCenter call; stays
// a no-op on non-darwin builds, which have no menu bar to show it from.
//
//nolint:gochecknoglobals // platform seam, see menubar_darwin.go's init.
var notify = func(title, body string) {}
