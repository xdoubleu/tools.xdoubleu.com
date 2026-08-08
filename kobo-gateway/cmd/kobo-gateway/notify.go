// Command kobo-gateway — notify is the seam between the platform-agnostic
// server (internal/kobogateway) and the menu bar's toast notifications, so
// self-update lifecycle events (#456) are visible even though
// internal/kobogateway has no AppKit dependency.
package main

// notify shows a best-effort menu-bar notification. nil until
// menubar_darwin.go's init assigns the real UNUserNotificationCenter call;
// stays nil on non-darwin builds, which have no menu bar to show it from.
// Server.SetNotifier (see server.go) already ignores a nil notifier, so
// callers pass this straight through without a nil check of their own.
//
//nolint:gochecknoglobals // platform seam, see menubar_darwin.go's init.
var notify func(title, body string)
