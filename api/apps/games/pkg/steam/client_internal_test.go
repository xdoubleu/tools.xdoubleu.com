package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

func newTestClient(
	t *testing.T,
	handler http.HandlerFunc,
	timeout, maxRetryWait time.Duration,
) client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return client{
		logger:       logging.NewNopLogger(),
		apiToken:     "test-key",
		baseURL:      srv.URL,
		http:         &http.Client{Timeout: timeout},
		retryWait:    time.Millisecond,
		maxRetryWait: maxRetryWait,
	}
}

func TestNew_SetsRequestTimeout(t *testing.T) {
	c, ok := New(logging.NewNopLogger(), "key").(client)
	require.True(t, ok)
	assert.Equal(t, requestTimeout, c.http.Timeout)
	assert.Positive(t, c.http.Timeout)
}

func TestSendRequestAPI_GivesUpAfterMaxAttemptsOf429(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}, time.Second, time.Millisecond)

	var dst struct{}
	err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

	require.ErrorContains(t, err, "429")
	assert.Equal(t, int32(maxAttempts), hits.Load())
}

func TestSendRequestAPI_RetriesAfterCappedRetryAfter(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, time.Second, 10*time.Millisecond)

	var dst struct {
		OK bool `json:"ok"`
	}
	start := time.Now()
	err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

	require.NoError(t, err)
	assert.True(t, dst.OK)
	assert.Equal(t, int32(2), hits.Load())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendRequestAPI_StopsWaitingOnContextCancel(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}, time.Second, time.Minute)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	var dst struct{}
	start := time.Now()
	err := c.sendRequestAPI(ctx, "endpoint", "", &dst)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), hits.Load())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendRequestAPI_TimesOut(t *testing.T) {
	release := make(chan struct{})
	c := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}, 20*time.Millisecond, time.Millisecond)
	defer close(release)

	var dst struct{}
	start := time.Now()
	err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestRetryAfter(t *testing.T) {
	const fallback, ceiling = 2 * time.Second, 30 * time.Second

	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"missing", "", fallback},
		{"seconds", "5", 5 * time.Second},
		{"zero", "0", 0},
		{"negative", "-3", 0},
		{"capped", "3600", ceiling},
		{"garbage", "soon", fallback},
		{
			"http date",
			time.Now().Add(time.Hour).UTC().Format(http.TimeFormat),
			ceiling,
		},
		{
			"past http date",
			time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat),
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, retryAfter(tt.header, fallback, ceiling))
		})
	}
}
