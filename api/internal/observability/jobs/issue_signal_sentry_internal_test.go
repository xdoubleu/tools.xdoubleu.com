package jobs

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/sentryapi"
)

func TestCollectSentryIssuesFetchOK(t *testing.T) {
	tests := []struct {
		name      string
		sentry    stubSentryClient
		wantCount float64
		wantOK    float64
	}{
		{
			name: "success sets count and ok",
			//nolint:exhaustruct //only the count of issues drives the gauge
			sentry:    stubSentryClient{issues: []sentryapi.Issue{{}, {}, {}}, err: nil},
			wantCount: 3,
			wantOK:    1,
		},
		{
			name:      "success with none is a trustworthy zero",
			sentry:    stubSentryClient{issues: nil, err: nil},
			wantCount: 0,
			wantOK:    1,
		},
		{
			name:      "not configured clears ok and keeps count",
			sentry:    stubSentryClient{issues: nil, err: sentryapi.ErrNotConfigured},
			wantCount: 5,
			wantOK:    0,
		},
		{
			name:      "fetch error clears ok and keeps count",
			sentry:    stubSentryClient{issues: nil, err: errors.New("boom")},
			wantCount: 5,
			wantOK:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGauges()
			sentryUnresolvedIssues.Set(5)
			sentryUnresolvedFetchOK.Set(1)
			//nolint:exhaustruct //only the sentry client is read
			job := &IssueSignalCollectorJob{sentry: tt.sentry}
			logger, _ := loggerWithBuf()
			job.collectSentryIssues(t.Context(), logger)
			assert.InDelta(t, tt.wantCount,
				testutil.ToFloat64(sentryUnresolvedIssues), 0)
			assert.InDelta(t, tt.wantOK,
				testutil.ToFloat64(sentryUnresolvedFetchOK), 0)
		})
	}
}
