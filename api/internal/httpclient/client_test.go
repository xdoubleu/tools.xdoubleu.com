package httpclient_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/httpclient"
	"tools.xdoubleu.com/internal/logging"
)

// fastConfig retries within milliseconds; tests change what they exercise.
func fastConfig() httpclient.Config {
	return httpclient.Config{
		Name:             "test",
		Logger:           logging.NewNopLogger(),
		UserAgent:        "",
		Timeout:          time.Second,
		MaxAttempts:      3,
		DeadlineAttempts: 0,
		BackoffBase:      time.Millisecond,
		BackoffCap:       10 * time.Millisecond,
		MaxRetryAfter:    20 * time.Millisecond,
		MaxBodyBytes:     0,
		Limiter:          nil,
		Retry:            nil,
		HTTPClient:       nil,
	}
}

// serve starts a server running h and counts the requests it handled.
func serve(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			h(w, r)
		},
	))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func statusHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("body"))
	}
}

func get(url string) *httpclient.Request {
	return httpclient.NewRequest(http.MethodGet, url)
}

func TestDo_RetryableStatuses(t *testing.T) {
	tests := map[string]struct {
		status       int
		wantAttempts int32
	}{
		"429 retried":               {http.StatusTooManyRequests, 3},
		"500 retried":               {http.StatusInternalServerError, 3},
		"502 retried":               {http.StatusBadGateway, 3},
		"503 retried":               {http.StatusServiceUnavailable, 3},
		"504 retried":               {http.StatusGatewayTimeout, 3},
		"599 retried":               {599, 3},
		"200 returned":              {http.StatusOK, 1},
		"400 returned":              {http.StatusBadRequest, 1},
		"401 returned":              {http.StatusUnauthorized, 1},
		"403 returned":              {http.StatusForbidden, 1},
		"404 returned":              {http.StatusNotFound, 1},
		"408 returned, not retried": {http.StatusRequestTimeout, 1},
		"499 returned":              {499, 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, statusHandler(tt.status))

			resp, err := httpclient.New(fastConfig()).Do(t.Context(), get(srv.URL))

			require.NoError(t, err, "an answered request is the caller's to judge")
			assert.Equal(t, tt.status, resp.StatusCode)
			assert.Equal(t, "body", string(resp.Body))
			assert.Equal(t, tt.wantAttempts, hits.Load())
		})
	}
}

func TestDo_RecoversAfterTransientStatus(t *testing.T) {
	var calls atomic.Int32
	srv, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Seen", "yes")
		_, _ = w.Write([]byte("ok"))
	})

	resp, err := httpclient.New(fastConfig()).Do(t.Context(), get(srv.URL))

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(resp.Body))
	assert.Equal(t, "yes", resp.Header.Get("X-Seen"))
	assert.Equal(t, int32(3), calls.Load())
}

func TestDo_ResendsMethodHeadersAndBodyOnEveryAttempt(t *testing.T) {
	var calls atomic.Int32
	srv, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, `{"q":1}`, string(got))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
		}
	})

	req := httpclient.NewRequest(http.MethodPost, srv.URL).
		SetHeader("Authorization", "Bearer tok").
		SetHeader("Content-Type", "application/json").
		SetBody([]byte(`{"q":1}`))
	resp, err := httpclient.New(fastConfig()).Do(t.Context(), req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load())
}

func TestDo_UserAgent(t *testing.T) {
	tests := map[string]struct {
		configured string
		perRequest string
		want       string
	}{
		"default":        {"", "", httpclient.DefaultUserAgent},
		"configured":     {"my-bot", "", "my-bot"},
		"request header": {"my-bot", "special", "special"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got atomic.Value
			srv, _ := serve(t, func(_ http.ResponseWriter, r *http.Request) {
				got.Store(r.Header.Get("User-Agent"))
			})
			cfg := fastConfig()
			cfg.UserAgent = tt.configured
			req := get(srv.URL)
			if tt.perRequest != "" {
				req.SetHeader("User-Agent", tt.perRequest)
			}

			_, err := httpclient.New(cfg).Do(t.Context(), req)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Load())
		})
	}
}

func TestDo_RetryAfter(t *testing.T) {
	tests := map[string]struct {
		header      string
		base        time.Duration
		cap         time.Duration
		maxRetry    time.Duration
		minElapsed  time.Duration
		wantUnderBy time.Duration
	}{
		"capped to MaxRetryAfter": {
			header: "3600", base: time.Hour, cap: time.Hour,
			maxRetry: 60 * time.Millisecond, minElapsed: 60 * time.Millisecond,
			wantUnderBy: 5 * time.Second,
		},
		"zero retries at once, ignoring backoff": {
			header: "0", base: time.Hour, cap: time.Hour,
			maxRetry: time.Hour, minElapsed: 0, wantUnderBy: 5 * time.Second,
		},
		"missing header uses backoff": {
			header: "", base: 50 * time.Millisecond, cap: time.Second,
			maxRetry: time.Hour, minElapsed: 50 * time.Millisecond,
			wantUnderBy: 5 * time.Second,
		},
		"garbage header uses backoff": {
			header: "soon", base: 50 * time.Millisecond, cap: time.Second,
			maxRetry: time.Hour, minElapsed: 50 * time.Millisecond,
			wantUnderBy: 5 * time.Second,
		},
		"past http date retries at once": {
			header:   time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat),
			base:     time.Hour,
			cap:      time.Hour,
			maxRetry: time.Hour, minElapsed: 0, wantUnderBy: 5 * time.Second,
		},
		"far http date capped": {
			header:   time.Now().Add(time.Hour).UTC().Format(http.TimeFormat),
			base:     time.Hour,
			cap:      time.Hour,
			maxRetry: 60 * time.Millisecond, minElapsed: 60 * time.Millisecond,
			wantUnderBy: 5 * time.Second,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					if tt.header != "" {
						w.Header().Set("Retry-After", tt.header)
					}
					w.WriteHeader(http.StatusTooManyRequests)
				}
			})
			cfg := fastConfig()
			cfg.BackoffBase, cfg.BackoffCap, cfg.MaxRetryAfter = tt.base, tt.cap, tt.maxRetry

			start := time.Now()
			resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))
			elapsed := time.Since(start)

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, int32(2), calls.Load())
			assert.GreaterOrEqual(t, elapsed, tt.minElapsed)
			assert.Less(t, elapsed, tt.wantUnderBy)
		})
	}
}

func TestDo_CancelledDuringBackoff(t *testing.T) {
	tests := map[string]string{
		"exponential backoff": "",
		"Retry-After wait":    "3600",
	}

	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				if header != "" {
					w.Header().Set("Retry-After", header)
				}
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			cfg := fastConfig()
			cfg.BackoffBase, cfg.BackoffCap = time.Hour, time.Hour
			cfg.MaxRetryAfter = time.Hour
			ctx, cancel := context.WithCancel(t.Context())
			time.AfterFunc(40*time.Millisecond, cancel)
			defer cancel()

			start := time.Now()
			resp, err := httpclient.New(cfg).Do(ctx, get(srv.URL))

			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, resp)
			assert.Equal(t, int32(1), hits.Load(), "no request after the cancel")
			assert.Less(t, time.Since(start), 5*time.Second)
		})
	}
}

func TestDo_SkipsRetryThatCannotFitBeforeDeadline(t *testing.T) {
	tests := map[string]struct {
		header       string
		wantAttempts int32
	}{
		"backoff longer than the time left":     {"", 1},
		"Retry-After longer than the time left": {"3600", 1},
		"Retry-After that fits is waited out":   {"0", 3},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.header != "" {
					w.Header().Set("Retry-After", tt.header)
				}
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			cfg := fastConfig()
			cfg.BackoffBase, cfg.BackoffCap = time.Hour, time.Hour
			cfg.MaxRetryAfter = time.Hour
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			start := time.Now()
			resp, err := httpclient.New(cfg).Do(ctx, get(srv.URL))

			require.NoError(t, err, "the last answer is handed back, not a ctx error")
			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
			assert.Equal(t, tt.wantAttempts, hits.Load())
			assert.Less(t, time.Since(start), 5*time.Second)
		})
	}
}

func TestDo_ContextAlreadyCancelled(t *testing.T) {
	srv, hits := serve(t, statusHandler(http.StatusOK))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := httpclient.New(fastConfig()).Do(ctx, get(srv.URL))

	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, hits.Load())
}

func TestDo_BodyCap(t *testing.T) {
	tests := map[string]struct {
		size    int
		max     int64
		status  int
		wantErr bool
	}{
		"under the cap":           {50, 100, http.StatusOK, false},
		"exactly at the cap":      {100, 100, http.StatusOK, false},
		"one byte over the cap":   {101, 100, http.StatusOK, true},
		"error body over the cap": {101, 100, http.StatusBadRequest, true},
		"default cap, at it": {
			int(httpclient.DefaultMaxBodyBytes), 0, http.StatusOK, false,
		},
		"default cap, over it": {
			int(httpclient.DefaultMaxBodyBytes) + 1, 0, http.StatusOK, true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(strings.Repeat("x", tt.size)))
			})
			cfg := fastConfig()
			cfg.MaxBodyBytes = tt.max

			resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

			assert.Equal(t, int32(1), hits.Load(), "an oversized body is not retried")
			if tt.wantErr {
				require.ErrorIs(t, err, httpclient.ErrBodyTooLarge)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Body, tt.size)
		})
	}
}

func TestDo_AttemptTimeout(t *testing.T) {
	t.Run("a timed-out attempt is retried", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		var calls atomic.Int32
		srv, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				select {
				case <-release:
				case <-r.Context().Done():
				}
				return
			}
			_, _ = w.Write([]byte("ok"))
		})
		cfg := fastConfig()
		cfg.Timeout = 30 * time.Millisecond

		resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

		require.NoError(t, err)
		assert.Equal(t, "ok", string(resp.Body))
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("timeouts that outlast the attempts surface as transient", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, hits := serve(t, func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		})
		cfg := fastConfig()
		cfg.Timeout = 20 * time.Millisecond

		_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

		require.Error(t, err)
		assert.True(t, httpclient.IsTransient(err))
		assert.Equal(t, int32(3), hits.Load())
	})

	t.Run("the caller's deadline stops further attempts", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, hits := serve(t, func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		})
		cfg := fastConfig()
		cfg.Timeout = 30 * time.Millisecond
		cfg.MaxAttempts = 100
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := httpclient.New(cfg).Do(ctx, get(srv.URL))

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, hits.Load(), int32(10))
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestDo_DeadlineAttempts(t *testing.T) {
	t.Run("a short deadline shares its time between fewer attempts", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		var calls atomic.Int32
		srv, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				select {
				case <-release:
				case <-r.Context().Done():
				}
				return
			}
			_, _ = w.Write([]byte("ok"))
		})
		cfg := fastConfig()
		cfg.Timeout = time.Minute
		cfg.MaxAttempts = 4
		cfg.DeadlineAttempts = 2
		ctx, cancel := context.WithTimeout(t.Context(), 600*time.Millisecond)
		defer cancel()

		resp, err := httpclient.New(cfg).Do(ctx, get(srv.URL))

		require.NoError(t, err, "the hung first attempt is cut at half the deadline")
		assert.Equal(t, "ok", string(resp.Body))
		assert.Equal(t, int32(2), calls.Load())
	})

	tests := map[string]struct {
		ctx          func(t *testing.T) context.Context
		wantAttempts int32
	}{
		"short deadline uses DeadlineAttempts": {
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				t.Cleanup(cancel)
				return ctx
			},
			wantAttempts: 2,
		},
		"deadline that fits every attempt keeps MaxAttempts": {
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
				t.Cleanup(cancel)
				return ctx
			},
			wantAttempts: 4,
		},
		"no deadline keeps MaxAttempts": {
			ctx:          func(t *testing.T) context.Context { return t.Context() },
			wantAttempts: 4,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, statusHandler(http.StatusServiceUnavailable))
			cfg := fastConfig()
			cfg.Timeout = 15 * time.Second
			cfg.MaxAttempts = 4
			cfg.DeadlineAttempts = 2

			_, err := httpclient.New(cfg).Do(tt.ctx(t), get(srv.URL))

			require.NoError(t, err)
			assert.Equal(t, tt.wantAttempts, hits.Load())
		})
	}
}

type countingLimiter struct {
	waits atomic.Int32
	err   error
}

func (l *countingLimiter) Wait(context.Context) error {
	l.waits.Add(1)
	return l.err
}

func TestDo_Limiter(t *testing.T) {
	t.Run("waits before every attempt", func(t *testing.T) {
		srv, hits := serve(t, statusHandler(http.StatusServiceUnavailable))
		limiter := new(countingLimiter)
		cfg := fastConfig()
		cfg.Limiter = limiter

		_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

		require.NoError(t, err)
		assert.Equal(t, int32(3), limiter.waits.Load())
		assert.Equal(t, int32(3), hits.Load())
	})

	t.Run("a limiter error ends the call without a request", func(t *testing.T) {
		srv, hits := serve(t, statusHandler(http.StatusOK))
		limiter := new(countingLimiter)
		limiter.err = errors.New("rate: would exceed deadline")
		cfg := fastConfig()
		cfg.Limiter = limiter

		_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

		require.ErrorIs(t, err, limiter.err)
		assert.Equal(t, int32(1), limiter.waits.Load())
		assert.Zero(t, hits.Load())
	})
}

func TestDo_CustomRetryPolicy(t *testing.T) {
	only429 := func(status int, _ error) bool {
		return status == http.StatusTooManyRequests
	}

	tests := map[string]struct {
		status       int
		wantAttempts int32
	}{
		"429 retried":                     {http.StatusTooManyRequests, 3},
		"503 not retried, policy decides": {http.StatusServiceUnavailable, 1},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, hits := serve(t, statusHandler(tt.status))
			cfg := fastConfig()
			cfg.Retry = only429

			resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
			assert.Equal(t, tt.wantAttempts, hits.Load())
		})
	}

	t.Run("policy also decides on transport errors", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		srv, hits := serve(t, func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		})
		cfg := fastConfig()
		cfg.Timeout = 20 * time.Millisecond
		cfg.Retry = only429

		_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

		require.Error(t, err)
		assert.Equal(t, int32(1), hits.Load())
	})
}

func TestDo_SingleAttempt(t *testing.T) {
	srv, hits := serve(t, statusHandler(http.StatusServiceUnavailable))
	cfg := fastConfig()
	cfg.MaxAttempts = 1

	resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, int32(1), hits.Load())
}

func TestDo_NonTransportErrorsAreNotRetried(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	url := srv.URL
	srv.Close() // connection refused

	tests := map[string]string{
		"connection refused": url,
		"unsupported scheme": "ftp://example.invalid/x",
		"unparseable url":    "http://[::1",
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			cfg := fastConfig()
			cfg.BackoffBase, cfg.BackoffCap = time.Hour, time.Hour

			resp, err := httpclient.New(cfg).Do(t.Context(), get(target))

			require.Error(t, err)
			assert.Nil(t, resp)
			assert.False(t, httpclient.IsTransient(err))
			assert.Less(t, time.Since(start), 5*time.Second, "no backoff was slept")
		})
	}
}

func TestDo_NilLoggerAndHTTPClientDefaults(t *testing.T) {
	srv, hits := serve(t, statusHandler(http.StatusServiceUnavailable))
	cfg := fastConfig()
	cfg.Logger = nil
	cfg.HTTPClient = nil

	_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

	require.NoError(t, err)
	assert.Equal(t, int32(3), hits.Load())
}

func TestDo_UsesTheGivenHTTPClient(t *testing.T) {
	srv, _ := serve(t, statusHandler(http.StatusOK))
	var used atomic.Bool
	cfg := fastConfig()
	cfg.HTTPClient = &http.Client{Transport: roundTripFunc(
		func(r *http.Request) (*http.Response, error) {
			used.Store(true)
			return http.DefaultTransport.RoundTrip(r)
		},
	)}

	_, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

	require.NoError(t, err)
	assert.True(t, used.Load())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSetBackoffBase_OverridesEveryClient(t *testing.T) {
	var calls atomic.Int32
	srv, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	cfg := fastConfig()
	cfg.BackoffBase, cfg.BackoffCap = time.Hour, time.Hour
	httpclient.SetBackoffBase(time.Millisecond)
	t.Cleanup(func() { httpclient.SetBackoffBase(0) })

	start := time.Now()
	resp, err := httpclient.New(cfg).Do(t.Context(), get(srv.URL))

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestIsRetryableStatus(t *testing.T) {
	tests := map[int]bool{
		http.StatusOK:                  false,
		http.StatusMovedPermanently:    false,
		http.StatusBadRequest:          false,
		http.StatusNotFound:            false,
		http.StatusRequestTimeout:      false,
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
		http.StatusServiceUnavailable:  true,
		599:                            true,
		600:                            false,
	}
	for status, want := range tests {
		assert.Equal(t, want, httpclient.IsRetryableStatus(status), status)
	}
}

func TestResponse_Snippet(t *testing.T) {
	short := &httpclient.Response{
		StatusCode: 500, Header: nil, Body: []byte("boom"),
	}
	long := &httpclient.Response{
		StatusCode: 500, Header: nil, Body: []byte(strings.Repeat("x", 5000)),
	}

	assert.Equal(t, "boom", short.Snippet())
	assert.Equal(t, strings.Repeat("x", 1024)+"...", long.Snippet())
}
