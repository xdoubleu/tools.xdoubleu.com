// Package httpclient is the retrying HTTP client behind the api's fixed-host
// integrations (GitHub, Sentry, Hardcover, UniCat, Steam). It bounds each
// attempt with a timeout, retries transient failures with exponential backoff
// or a capped Retry-After, stops when the caller's context ends, and caps
// response bodies. Fetching user-supplied URLs goes through internal/webfetch.
package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	// DefaultUserAgent identifies the api to upstream services.
	DefaultUserAgent     = "tools.xdoubleu.com"
	DefaultTimeout       = 15 * time.Second
	DefaultMaxAttempts   = 4
	DefaultBackoffBase   = 500 * time.Millisecond
	DefaultBackoffCap    = 30 * time.Second
	DefaultMaxRetryAfter = 30 * time.Second
	// DefaultMaxBodyBytes bounds a response body when Config.MaxBodyBytes is 0.
	DefaultMaxBodyBytes = int64(10 << 20)

	// snippetBytes bounds how much of a body an error message quotes.
	snippetBytes = 1024
)

// ErrBodyTooLarge is returned when a response body exceeds the size cap.
var ErrBodyTooLarge = errors.New("httpclient: response exceeds size limit")

//nolint:gochecknoglobals // test hook behind SetBackoffBase
var backoffBaseOverride atomic.Int64

// SetBackoffBase makes every client use d as its backoff base, so tests retry
// within milliseconds; 0 restores each client's own. Tests only.
func SetBackoffBase(d time.Duration) { backoffBaseOverride.Store(int64(d)) }

// Limiter paces requests; *rate.Limiter satisfies it.
type Limiter interface {
	Wait(ctx context.Context) error
}

// Config tunes a Client. Zero fields take the Default* values, except Logger,
// Limiter and HTTPClient, which are optional.
type Config struct {
	// Name labels the client in retry logs.
	Name   string
	Logger *slog.Logger
	// UserAgent is sent unless the request sets its own.
	UserAgent string
	// Timeout bounds one attempt, including reading its body.
	Timeout time.Duration
	// MaxAttempts is the number of tries per Do, the first included.
	MaxAttempts int
	// DeadlineAttempts replaces MaxAttempts when the caller's deadline is
	// shorter than MaxAttempts full attempts; each attempt then gets an even
	// share of the time left, so a retry still fits. 0 disables it.
	DeadlineAttempts int
	// BackoffBase doubles with each retry up to BackoffCap.
	BackoffBase time.Duration
	BackoffCap  time.Duration
	// MaxRetryAfter caps how long a Retry-After header can make Do wait.
	MaxRetryAfter time.Duration
	// MaxBodyBytes caps a response body.
	MaxBodyBytes int64
	// Limiter, when set, is waited on before every attempt.
	Limiter Limiter
	// Retry decides whether an attempt is retried: status is 0 when err is set.
	// nil uses [ShouldRetry].
	Retry func(status int, err error) bool
	// HTTPClient sends the requests; nil uses a plain http.Client.
	HTTPClient *http.Client
}

// Client sends requests per its Config. It is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a Client for cfg.
func New(cfg Config) *Client {
	cfg.UserAgent = orDefault(cfg.UserAgent, DefaultUserAgent)
	cfg.Timeout = orDefault(cfg.Timeout, DefaultTimeout)
	cfg.MaxAttempts = orDefault(cfg.MaxAttempts, DefaultMaxAttempts)
	cfg.BackoffBase = orDefault(cfg.BackoffBase, DefaultBackoffBase)
	cfg.BackoffCap = orDefault(cfg.BackoffCap, DefaultBackoffCap)
	cfg.MaxRetryAfter = orDefault(cfg.MaxRetryAfter, DefaultMaxRetryAfter)
	cfg.MaxBodyBytes = orDefault(cfg.MaxBodyBytes, DefaultMaxBodyBytes)
	cfg.DeadlineAttempts = min(cfg.DeadlineAttempts, cfg.MaxAttempts)
	if cfg.Retry == nil {
		cfg.Retry = ShouldRetry
	}

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{cfg: cfg, http: hc}
}

func orDefault[T comparable](v, fallback T) T {
	var zero T
	if v == zero {
		return fallback
	}
	return v
}

// Request is a call to send; build it with [NewRequest].
type Request struct {
	method string
	rawURL string
	header http.Header
	body   []byte
}

// NewRequest returns a request without headers or body.
func NewRequest(method, rawURL string) *Request {
	return &Request{
		method: method,
		rawURL: rawURL,
		header: http.Header{},
		body:   nil,
	}
}

// SetHeader sets a request header.
func (r *Request) SetHeader(key, value string) *Request {
	r.header.Set(key, value)
	return r
}

// SetBody sets the body, resent in full on every attempt.
func (r *Request) SetBody(body []byte) *Request {
	r.body = body
	return r
}

// Response is a completed exchange with its whole body read.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Snippet returns the start of the body for quoting in an error message.
func (r *Response) Snippet() string {
	if len(r.Body) <= snippetBytes {
		return string(r.Body)
	}
	return string(r.Body[:snippetBytes]) + "..."
}

// Do sends req and returns the server's answer. A non-nil Response means the
// server answered; judging its status is the caller's job, including a
// retryable one that outlasted the attempts or the caller's deadline. The
// error is for requests that never got an answer: transport failures, an
// oversized body, a limiter or context error.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	attempts, bounded := c.attempts(ctx)

	for try := 1; ; try++ {
		if c.cfg.Limiter != nil {
			if err := c.cfg.Limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}

		timeout := c.attemptTimeout(ctx, bounded, attempts-try+1)
		resp, err := c.once(ctx, req, timeout)

		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if try >= attempts || ctx.Err() != nil || !c.cfg.Retry(status, err) {
			return resp, err
		}

		delay := c.delay(try-1, resp)
		if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
			// Waiting would burn what's left of the caller's budget on a retry
			// that can't run; hand back what we have while it can still answer.
			return resp, err
		}
		c.logRetry(ctx, try, delay, status, err)
		if sleepErr := sleep(ctx, delay); sleepErr != nil {
			return nil, sleepErr
		}
	}
}

func (c *Client) once(
	ctx context.Context, req *Request, timeout time.Duration,
) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if req.body != nil {
		body = bytes.NewReader(req.body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.method, req.rawURL, body)
	if err != nil {
		return nil, err
	}
	httpReq.Header = req.header.Clone()
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", c.cfg.UserAgent)
	}

	res, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Read one byte past the cap to tell exactly-at-cap from over.
	data, err := io.ReadAll(io.LimitReader(res.Body, c.cfg.MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.cfg.MaxBodyBytes {
		return nil, fmt.Errorf(
			"%w: over %d bytes", ErrBodyTooLarge, c.cfg.MaxBodyBytes,
		)
	}
	return &Response{
		StatusCode: res.StatusCode,
		Header:     res.Header,
		Body:       data,
	}, nil
}

// attempts returns how many tries Do makes and whether the caller's deadline
// forces them to share its remaining time.
func (c *Client) attempts(ctx context.Context) (int, bool) {
	deadline, ok := ctx.Deadline()
	if ok && c.cfg.DeadlineAttempts > 0 &&
		time.Until(deadline) < time.Duration(c.cfg.MaxAttempts)*c.cfg.Timeout {
		return c.cfg.DeadlineAttempts, true
	}
	return c.cfg.MaxAttempts, false
}

// attemptTimeout is Timeout, or in bounded mode an even share of the time
// left for the attempts still to make (this one included).
func (c *Client) attemptTimeout(
	ctx context.Context, bounded bool, left int,
) time.Duration {
	if !bounded {
		return c.cfg.Timeout
	}
	deadline, _ := ctx.Deadline()
	return min(c.cfg.Timeout, time.Until(deadline)/time.Duration(left))
}
