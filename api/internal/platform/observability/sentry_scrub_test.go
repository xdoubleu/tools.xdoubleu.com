package observability_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/observability"
)

func TestScrubSentryEvent_RedactsKoboToken(t *testing.T) {
	//nolint:exhaustruct // only the scrubbed fields matter
	event := &sentry.Event{
		Transaction: "GET /api/books/kobo/secret-token/v1/library/sync",
		Request: &sentry.Request{ //nolint:exhaustruct // only URL fields matter
			URL:         "https://tools.xdoubleu.com/api/books/kobo/secret-token/v1/x",
			QueryString: "a=b",
		},
		Spans: []*sentry.Span{
			//nolint:exhaustruct // description only
			{Description: "GET /api/books/kobo/secret-token/v1/x"},
		},
	}

	got := observability.ScrubSentryEvent(event, nil)

	assert.Equal(t, "GET /api/books/kobo/redacted/v1/library/sync", got.Transaction)
	assert.Equal(
		t, "https://tools.xdoubleu.com/api/books/kobo/redacted/v1/x", got.Request.URL,
	)
	assert.Empty(t, got.Request.QueryString)
	assert.Equal(t, "GET /api/books/kobo/redacted/v1/x", got.Spans[0].Description)
}

func TestScrubSentryEvent_NoRequest(t *testing.T) {
	event := &sentry.Event{Transaction: "GET /health"} //nolint:exhaustruct // no request
	assert.Equal(t, "GET /health", observability.ScrubSentryEvent(event, nil).Transaction)
}
