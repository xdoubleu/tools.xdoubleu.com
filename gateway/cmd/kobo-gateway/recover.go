// Command kobo-gateway — panic recovery shared by darwin and non-darwin
// builds (menu-bar goroutines are darwin-only, but the helper itself doesn't
// touch AppKit, so it lives outside the build-tagged files).
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// sentryFlushTimeout bounds how long a recovered panic gets to reach Sentry
// before the caller continues (or the process exits).
const sentryFlushTimeout = 2 * time.Second

// guard recovers a panic in its caller's deferred context, logs it to
// stderr, and reports it to Sentry (a no-op if Sentry was never initialized —
// see initSentry), then lets execution continue past the panic. Use it on
// per-event/per-block work (a goroutine loop iteration, a dispatched menu
// update) where one bad event shouldn't take down the whole app.
//
// This is defense-in-depth for pure Go panics only: launchd's KeepAlive (see
// internal/kobogateway/loginitem.go) is what recovers the process from the
// darwinkit ObjC bridge's SIGABRT, which no Go recover() can catch.
func guard(where string) {
	r := recover()
	if r == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "kobo-gateway: recovered panic in %s: %v\n", where, r)

	sentry.CurrentHub().Recover(r)
	sentry.Flush(sentryFlushTimeout)
}

// recoverGo runs fn with guard deferred, for use as a goroutine body, e.g.
// `go recoverGo("watchKobos", watchKobosOnce)`.
func recoverGo(where string, fn func()) {
	defer guard(where)

	fn()
}

// reportAndRepanic recovers a panic on the main thread, reports it to
// Sentry, and re-panics. Unlike guard, it does not swallow the panic:
// silently continuing past a broken AppKit run loop would leave the app
// running in an unusable, un-relaunched state, whereas re-panicking exits
// non-zero and lets launchd's KeepAlive relaunch a fresh process.
func reportAndRepanic() {
	r := recover()
	if r == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "kobo-gateway: panic: %v\n", r)

	sentry.CurrentHub().Recover(r)
	sentry.Flush(sentryFlushTimeout)

	panic(r)
}
