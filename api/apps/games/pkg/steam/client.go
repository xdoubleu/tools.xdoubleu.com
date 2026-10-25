package steam

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	httptools "tools.xdoubleu.com/internal/communication/httptools"
)

const BaseURLRESTAPI = "http://api.steampowered.com"

const (
	requestTimeout = 15 * time.Second
	// maxAttempts bounds the tries of one request that keeps getting a 429.
	maxAttempts = 3
	// retryWait applies when a 429 carries no Retry-After.
	retryWait    = 10 * time.Second
	maxRetryWait = 30 * time.Second
)

type client struct {
	logger       *slog.Logger
	apiToken     string
	baseURL      string
	http         *http.Client
	retryWait    time.Duration
	maxRetryWait time.Duration
}

func New(logger *slog.Logger, apiToken string) Client {
	return client{
		logger:       logger,
		apiToken:     apiToken,
		baseURL:      BaseURLRESTAPI,
		http:         &http.Client{Timeout: requestTimeout},
		retryWait:    retryWait,
		maxRetryWait: maxRetryWait,
	}
}

// sendRequestAPI retries a 429 up to maxAttempts, waiting its capped
// Retry-After or until ctx ends.
func (client client) sendRequestAPI(
	ctx context.Context,
	endpoint string,
	query string,
	dst any,
) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s", client.baseURL, endpoint))
	if err != nil {
		return err
	}

	u.RawQuery = query
	tempQuery := u.Query()
	tempQuery.Add("key", client.apiToken)
	u.RawQuery = tempQuery.Encode()

	for attempt := 1; ; attempt++ {
		err = client.sendRequest(ctx, u.String(), dst)
		var tooMany tooManyRequestsError
		if !errors.As(err, &tooMany) || attempt == maxAttempts {
			return err
		}

		client.logger.DebugContext(ctx, "hit 429, retrying",
			slog.Duration("wait", tooMany.wait), slog.Int("attempt", attempt))
		timer := time.NewTimer(tooMany.wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// tooManyRequestsError is a 429 and how long to wait before retrying it.
type tooManyRequestsError struct {
	wait time.Duration
}

func (e tooManyRequestsError) Error() string {
	return "request failed with status code 429"
}

func (client client) sendRequest(ctx context.Context, rawURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	res, err := client.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return tooManyRequestsError{wait: retryAfter(
			res.Header.Get("Retry-After"),
			client.retryWait,
			client.maxRetryWait,
		)}
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var bodyBytes []byte
		bodyBytes, err = io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf(
				"request failed with status code %d and failed to read response body; error: %w",
				res.StatusCode,
				err,
			)
		}

		return fmt.Errorf(
			"request failed with status code %d and response body: %s",
			res.StatusCode,
			string(bodyBytes),
		)
	}

	return httptools.ReadJSON(res.Body, dst)
}

// retryAfter reads a Retry-After header (seconds or HTTP date), falling back
// when it is missing or malformed, and clamps it to [0, ceiling].
func retryAfter(header string, fallback, ceiling time.Duration) time.Duration {
	wait := fallback
	if secs, err := strconv.Atoi(header); err == nil {
		wait = time.Duration(secs) * time.Second
	} else if at, dateErr := http.ParseTime(header); dateErr == nil {
		wait = time.Until(at)
	}
	return max(min(wait, ceiling), 0)
}
