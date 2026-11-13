package httpclient

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ShouldRetry is the default Config.Retry: transient transport errors and
// retryable statuses.
func ShouldRetry(status int, err error) bool {
	if err != nil {
		return IsTransient(err)
	}
	return IsRetryableStatus(status)
}

// IsRetryableStatus reports whether status is a 429 or a 5xx.
func IsRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		(status >= http.StatusInternalServerError && status < 600)
}

// IsTransient reports whether err is self-healing: a timeout, an expired
// deadline or a DNS lookup failure. A cancellation is not.
func IsTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// DNS failures (e.g. a flaky Docker resolver) clear up on their own.
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// delay is how long to wait before retry number retry+1: the response's
// Retry-After when it has a usable one, else exponential backoff.
func (c *Client) delay(retry int, resp *Response) time.Duration {
	if resp != nil {
		if wait, ok := retryAfter(
			resp.Header.Get("Retry-After"), c.cfg.MaxRetryAfter,
		); ok {
			return wait
		}
	}
	return c.backoff(retry)
}

func (c *Client) backoff(retry int) time.Duration {
	d := c.cfg.BackoffBase
	if override := time.Duration(backoffBaseOverride.Load()); override > 0 {
		d = override
	}
	for range retry {
		if d >= c.cfg.BackoffCap {
			break
		}
		d *= 2
	}
	return min(d, c.cfg.BackoffCap)
}

// retryAfter reads a Retry-After header (seconds or HTTP date) clamped to
// [0, ceiling]. ok is false when it is missing or malformed.
func retryAfter(header string, ceiling time.Duration) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(header); err == nil {
		if secs <= 0 {
			return 0, true
		}
		if int64(secs) > int64(ceiling/time.Second) {
			return ceiling, true
		}
		return min(time.Duration(secs)*time.Second, ceiling), true
	}
	if at, err := http.ParseTime(header); err == nil {
		return max(min(time.Until(at), ceiling), 0), true
	}
	return 0, false
}

func (c *Client) logRetry(
	ctx context.Context, try int, delay time.Duration, status int, err error,
) {
	if c.cfg.Logger == nil {
		return
	}
	c.cfg.Logger.DebugContext(ctx, "retrying request",
		slog.String("client", c.cfg.Name),
		slog.Int("attempt", try),
		slog.Duration("backoff", delay),
		slog.Int("status", status),
		slog.Any("error", err),
	)
}

// sleep waits d, or returns the context's error once it ends.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
