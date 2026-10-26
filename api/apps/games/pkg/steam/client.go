package steam

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	httptools "tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/httpclient"
)

const BaseURLRESTAPI = "http://api.steampowered.com"

const (
	requestTimeout = 15 * time.Second
	// maxAttempts bounds the tries of one request that keeps getting a 429.
	maxAttempts = 3
	// retryWait is the wait after a 429 without a Retry-After; it doubles.
	retryWait    = 10 * time.Second
	maxRetryWait = 30 * time.Second
)

type client struct {
	apiToken string
	baseURL  string
	http     *httpclient.Client
}

func New(logger *slog.Logger, apiToken string) Client {
	return client{
		apiToken: apiToken,
		baseURL:  BaseURLRESTAPI,
		http:     httpclient.New(httpConfig(logger)),
	}
}

// httpConfig retries only a 429, waiting its capped Retry-After; a timeout or
// any other status is final.
func httpConfig(logger *slog.Logger) httpclient.Config {
	return httpclient.Config{
		Name:          "steam",
		Logger:        logger,
		Timeout:       requestTimeout,
		MaxAttempts:   maxAttempts,
		BackoffBase:   retryWait,
		BackoffCap:    maxRetryWait,
		MaxRetryAfter: maxRetryWait,
		Retry: func(status int, _ error) bool {
			return status == http.StatusTooManyRequests
		},
	}
}

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

	res, err := client.http.Do(ctx, httpclient.NewRequest(http.MethodGet, u.String()))
	if err != nil {
		return err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"request failed with status code %d and response body: %s",
			res.StatusCode,
			res.Snippet(),
		)
	}

	return httptools.ReadJSON(bytes.NewReader(res.Body), dst)
}
