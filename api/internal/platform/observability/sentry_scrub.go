package observability

import (
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/internal/platform/middleware"
)

// isSpanless reports job transactions whose per-item spans are the work itself
// (a GitHub read per open pull request), which Sentry's N+1 detector reports as
// a performance issue. Their spans are dropped; duration and status stay.
func isSpanless(transaction string) bool {
	return transaction == "factory-poll-pull-requests"
}

// ScrubSentryEvent removes credentials carried in request paths (the Kobo
// device token) from transaction names, request URLs and span descriptions, and
// drops the spans of [isSpanless] transactions.
func ScrubSentryEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.Transaction = middleware.RedactPath(event.Transaction)
	if event.Request != nil {
		event.Request.URL = middleware.RedactPath(event.Request.URL)
		event.Request.QueryString = ""
	}
	for _, span := range event.Spans {
		span.Description = middleware.RedactPath(span.Description)
	}
	if isSpanless(event.Transaction) {
		event.Spans = nil
	}
	return event
}
