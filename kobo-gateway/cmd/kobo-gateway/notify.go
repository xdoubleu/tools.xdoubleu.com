// Command kobo-gateway — notify bridges internal/kobogateway (no AppKit) to
// the menu bar's notifications.
package main

// notify shows a best-effort notification. Set by menubar_darwin.go's init;
// nil elsewhere, which Server.SetNotifier ignores.
//
//nolint:gochecknoglobals // platform seam, see menubar_darwin.go's init.
var notify func(title, body string)
