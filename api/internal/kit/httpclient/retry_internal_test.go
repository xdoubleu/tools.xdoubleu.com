package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryAfter(t *testing.T) {
	const ceiling = 30 * time.Second

	tests := []struct {
		name   string
		header string
		want   time.Duration
		wantOK bool
	}{
		{"missing", "", 0, false},
		{"blank", "  ", 0, false},
		{"seconds", "5", 5 * time.Second, true},
		{"padded seconds", " 5 ", 5 * time.Second, true},
		{"zero", "0", 0, true},
		{"negative", "-3", 0, true},
		{"capped", "3600", ceiling, true},
		{"overflowing seconds", "9999999999999999", ceiling, true},
		{"out of int range", "99999999999999999999", 0, false},
		{"garbage", "soon", 0, false},
		{
			"http date capped",
			time.Now().Add(time.Hour).UTC().Format(http.TimeFormat),
			ceiling, true,
		},
		{
			"past http date",
			time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat),
			0, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := retryAfter(tt.header, ceiling)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("http date within the cap", func(t *testing.T) {
		at := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)

		got, ok := retryAfter(at, ceiling)

		assert.True(t, ok)
		assert.InDelta(t, float64(10*time.Second), float64(got), float64(2*time.Second))
	})

	t.Run("sub-second ceiling still caps whole seconds", func(t *testing.T) {
		got, ok := retryAfter("2", 50*time.Millisecond)

		assert.True(t, ok)
		assert.Equal(t, 50*time.Millisecond, got)
	})
}

func TestBackoff(t *testing.T) {
	c := New(Config{
		BackoffBase: 100 * time.Millisecond,
		BackoffCap:  time.Second,
	})

	tests := []struct {
		retry int
		want  time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, time.Second},
		{5, time.Second},
		{1000, time.Second},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("retry %d", tt.retry), func(t *testing.T) {
			assert.Equal(t, tt.want, c.backoff(tt.retry))
		})
	}
}

func TestNew_FillsDefaults(t *testing.T) {
	c := New(Config{})

	assert.Equal(t, DefaultUserAgent, c.cfg.UserAgent)
	assert.Equal(t, DefaultTimeout, c.cfg.Timeout)
	assert.Equal(t, DefaultMaxAttempts, c.cfg.MaxAttempts)
	assert.Equal(t, DefaultBackoffBase, c.cfg.BackoffBase)
	assert.Equal(t, DefaultBackoffCap, c.cfg.BackoffCap)
	assert.Equal(t, DefaultMaxRetryAfter, c.cfg.MaxRetryAfter)
	assert.Equal(t, DefaultMaxBodyBytes, c.cfg.MaxBodyBytes)
	assert.NotNil(t, c.cfg.Retry)
	assert.NotNil(t, c.http)
}

func TestNew_ClampsDeadlineAttemptsToMaxAttempts(t *testing.T) {
	c := New(Config{
		MaxAttempts:      2,
		DeadlineAttempts: 9,
	})

	assert.Equal(t, 2, c.cfg.DeadlineAttempts)
}

func TestIsTransient(t *testing.T) {
	timeout := &url.Error{Op: "Get", URL: "u", Err: timeoutError{}}

	tests := map[string]struct {
		err  error
		want bool
	}{
		"nil":               {nil, false},
		"cancelled":         {context.Canceled, false},
		"wrapped cancelled": {fmt.Errorf("x: %w", context.Canceled), false},
		"deadline":          {context.DeadlineExceeded, true},
		"wrapped deadline":  {fmt.Errorf("x: %w", context.DeadlineExceeded), true},
		"url error timeout": {timeout, true},
		"url error, not a timeout": {
			&url.Error{Op: "Get", URL: "u", Err: errors.New("boom")}, false,
		},
		"dns failure":         {dnsError(), true},
		"wrapped dns failure": {fmt.Errorf("x: %w", dnsError()), true},
		"body too large":      {ErrBodyTooLarge, false},
		"arbitrary error":     {errors.New("boom"), false},
		"cancelled timeout": {
			&url.Error{Op: "Get", URL: "u", Err: context.Canceled}, false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTransient(tt.err))
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := map[string]struct {
		status int
		err    error
		want   bool
	}{
		"503":                  {http.StatusServiceUnavailable, nil, true},
		"404":                  {http.StatusNotFound, nil, false},
		"200":                  {http.StatusOK, nil, false},
		"transient error":      {0, context.DeadlineExceeded, true},
		"permanent error":      {0, errors.New("boom"), false},
		"error beats a status": {http.StatusServiceUnavailable, errors.New("boom"), false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldRetry(tt.status, tt.err))
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func dnsError() *net.DNSError {
	//nolint:exhaustruct // only the fields under test
	return &net.DNSError{Err: "no such host", Name: "h"}
}
