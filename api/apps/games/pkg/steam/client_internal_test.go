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

	"tools.xdoubleu.com/internal/httpclient"
	"tools.xdoubleu.com/internal/logging"
)

// newTestClient is the production client with its waits and timeout shortened.
func newTestClient(
	t *testing.T,
	handler http.HandlerFunc,
	timeout, maxRetryWait time.Duration,
) client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := httpConfig(logging.NewNopLogger())
	cfg.Timeout = timeout
	cfg.BackoffBase = time.Millisecond
	cfg.BackoffCap = maxRetryWait
	cfg.MaxRetryAfter = maxRetryWait
	return client{
		apiToken: "test-key",
		baseURL:  srv.URL,
		http:     httpclient.New(cfg),
	}
}

func TestHTTPConfig(t *testing.T) {
	cfg := httpConfig(logging.NewNopLogger())

	assert.Equal(t, requestTimeout, cfg.Timeout)
	assert.Equal(t, maxAttempts, cfg.MaxAttempts)
	assert.Equal(t, maxRetryWait, cfg.MaxRetryAfter)
	assert.True(t, cfg.Retry(http.StatusTooManyRequests, nil))
	assert.False(t, cfg.Retry(http.StatusInternalServerError, nil), "5xx is final")
	assert.False(t, cfg.Retry(0, context.DeadlineExceeded), "a timeout is final")
}

func TestNew_BuildsAClient(t *testing.T) {
	c, ok := New(logging.NewNopLogger(), "key").(client)

	require.True(t, ok)
	assert.Equal(t, "key", c.apiToken)
	assert.Equal(t, BaseURLRESTAPI, c.baseURL)
	assert.NotNil(t, c.http)
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

func TestSendRequestAPI_DoesNotWaitPastTheDeadline(t *testing.T) {
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

	require.ErrorContains(t, err, "429")
	assert.Equal(t, int32(1), hits.Load())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendRequestAPI_StopsWaitingOnContextCancel(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}, time.Second, time.Minute)

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)
	defer cancel()

	var dst struct{}
	start := time.Now()
	err := c.sendRequestAPI(ctx, "endpoint", "", &dst)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), hits.Load())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendRequestAPI_TimeoutIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	release := make(chan struct{})
	c := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
		hits.Add(1)
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
	assert.Equal(t, int32(1), hits.Load())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendRequestAPI_OtherStatusesAreNotRetried(t *testing.T) {
	for _, status := range []int{
		http.StatusForbidden,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var hits atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				http.Error(w, "nope", status)
			}, time.Second, time.Millisecond)

			var dst struct{}
			err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

			require.ErrorContains(t, err, "nope")
			assert.Equal(t, int32(1), hits.Load())
		})
	}
}

func TestSendRequestAPI_SendsTheKeyAndQuery(t *testing.T) {
	var got atomic.Value
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.URL.Path + "?" + r.URL.Query().Encode())
		_, _ = w.Write([]byte(`{}`))
	}, time.Second, time.Millisecond)

	var dst struct{}
	err := c.sendRequestAPI(t.Context(), "ISteamUser/Thing", "steamid=7", &dst)

	require.NoError(t, err)
	assert.Equal(t, "/ISteamUser/Thing?key=test-key&steamid=7", got.Load())
}

func TestSendRequestAPI_RejectsBadJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}, time.Second, time.Millisecond)

	var dst struct{}
	err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

	require.Error(t, err)
}

func TestSendRequestAPI_BadEndpoint(t *testing.T) {
	c := client{
		apiToken: "k",
		baseURL:  "http://[::1",
		http:     httpclient.New(httpclient.Config{}),
	}

	var dst struct{}
	err := c.sendRequestAPI(t.Context(), "endpoint", "", &dst)

	require.Error(t, err)
}
