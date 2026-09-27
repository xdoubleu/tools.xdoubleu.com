package main

import (
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/internal/middleware"
)

// scrubSentryEvent removes credentials carried in request paths (the Kobo
// device token) from transaction names, request URLs and span descriptions.
func scrubSentryEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.Transaction = middleware.RedactPath(event.Transaction)
	if event.Request != nil {
		event.Request.URL = middleware.RedactPath(event.Request.URL)
		event.Request.QueryString = ""
	}
	for _, span := range event.Spans {
		span.Description = middleware.RedactPath(span.Description)
	}
	return event
}
