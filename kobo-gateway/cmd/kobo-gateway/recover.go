// Command kobo-gateway — panic recovery shared by darwin and non-darwin builds.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// sentryFlushTimeout bounds how long a recovered panic gets to reach Sentry.
const sentryFlushTimeout = 2 * time.Second

// guard recovers a panic in its caller's deferred context, logs and reports
// it, and continues. For per-event work where one bad event shouldn't crash
// the app. Go panics only: launchd's KeepAlive handles the ObjC SIGABRT.
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

// reportFatal reports a fatal non-panic error from run() before main exits;
// a menu-bar app otherwise leaves no trace.
func reportFatal(err error) {
	sentry.CurrentHub().CaptureException(err)
	sentry.Flush(sentryFlushTimeout)
}

// reportAndRepanic recovers a main-thread panic, reports it, and re-panics so
// launchd's KeepAlive relaunches a fresh process.
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
