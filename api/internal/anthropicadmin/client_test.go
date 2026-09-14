package anthropicadmin_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/anthropicadmin"
	"tools.xdoubleu.com/internal/logging"
)

const realBaseURL = "https://api.anthropic.com"

func TestMain(m *testing.M) {
	anthropicadmin.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

// buildServer starts an httptest.Server serving handler and points the
// package-level baseURL at it. The returned func restores the real URL.
func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	anthropicadmin.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		anthropicadmin.SetBaseURL(realBaseURL)
	}
}

func newClient(apiKey string) anthropicadmin.Client {
	return anthropicadmin.New(logging.NewNopLogger(), apiKey)
}

func TestGetClaudeCodeUsage_NotConfigured(t *testing.T) {
	c := newClient("")
	_, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
	require.ErrorIs(t, err, anthropicadmin.ErrNotConfigured)
}

func TestGetClaudeCodeUsage_SinglePage(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-key", r.Header.Get("X-Api-Key"))
			assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
			assert.Equal(t, "2026-09-01", r.URL.Query().Get("starting_at"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"customer_type": "subscription",
						"date": "2026-09-01T00:00:00Z",
						"model_breakdown": [
							{
								"model": "claude-sonnet-5",
								"tokens": {"input": 100, "output": 50, "cache_creation": 10, "cache_read": 5},
								"estimated_cost": {"amount": 42, "currency": "USD"}
							}
						]
					}
				],
				"has_more": false,
				"next_page": null
			}`))
		},
	))
	defer cleanup()

	c := newClient("test-key")
	records, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "subscription", records[0].CustomerType)
	require.Len(t, records[0].ModelBreakdown, 1)
	assert.Equal(t, "claude-sonnet-5", records[0].ModelBreakdown[0].Model)
	assert.InEpsilon(t, float64(100), records[0].ModelBreakdown[0].Tokens.Input, 0)
	assert.InEpsilon(
		t,
		float64(42),
		records[0].ModelBreakdown[0].EstimatedCost.Amount,
		0,
	)
}

func TestGetClaudeCodeUsage_FollowsPagination(t *testing.T) {
	pages := []string{
		`{"data":[{"customer_type":"api","date":"2026-09-01T00:00:00Z",` +
			`"model_breakdown":[]}],"has_more":true,"next_page":"cursor-2"}`,
		`{"data":[{"customer_type":"subscription","date":"2026-09-01T00:00:00Z",` +
			`"model_breakdown":[]}],"has_more":false,"next_page":null}`,
	}
	call := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if call == 0 {
				assert.Empty(t, r.URL.Query().Get("page"))
			} else {
				assert.Equal(t, "cursor-2", r.URL.Query().Get("page"))
			}
			_, _ = w.Write([]byte(pages[call]))
			call++
		},
	))
	defer cleanup()

	c := newClient("test-key")
	records, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "api", records[0].CustomerType)
	assert.Equal(t, "subscription", records[1].CustomerType)
	assert.Equal(t, 2, call)
}

func TestGetClaudeCodeUsage_RetriesOn5xxThenSucceeds(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[],"has_more":false,"next_page":null}`))
		},
	))
	defer cleanup()

	c := newClient("test-key")
	records, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
	require.NoError(t, err)
	assert.Empty(t, records)
	assert.Equal(t, 2, attempts)
}

func TestGetClaudeCodeUsage_NonRetryableStatusFailsImmediately(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
		},
	))
	defer cleanup()

	c := newClient("bad-key")
	_, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "invalid api key")
	assert.Equal(t, 1, attempts)
	assert.False(t, anthropicadmin.IsTransientAPIError(err))
}

func TestIsTransientAPIError(t *testing.T) {
	t.Run("plain error is not transient", func(t *testing.T) {
		assert.False(t, anthropicadmin.IsTransientAPIError(errors.New("boom")))
	})
	t.Run(
		"5xx response surfaces as a transient error after retries exhaust",
		func(t *testing.T) {
			cleanup := buildServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
				},
			))
			defer cleanup()

			c := newClient("test-key")
			_, err := c.GetClaudeCodeUsage(t.Context(), "2026-09-01")
			require.Error(t, err)
			assert.True(t, anthropicadmin.IsTransientAPIError(err))
		},
	)
}
