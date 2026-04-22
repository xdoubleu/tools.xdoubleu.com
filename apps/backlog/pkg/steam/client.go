package steam

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
)

const BaseURLRESTAPI = "http://api.steampowered.com"
const StorefrontURLRESTAPI = "https://store.steampowered.com/api"

type client struct {
	logger   *slog.Logger
	apiToken string
}

func New(logger *slog.Logger, apiToken string) Client {
	return client{
		logger:   logger,
		apiToken: apiToken,
	}
}

func (client client) sendRequestAPI(
	ctx context.Context,
	endpoint string,
	query string,
	dst any,
) error {
	return sendRequest(
		ctx,
		client.logger,
		client.apiToken,
		BaseURLRESTAPI,
		endpoint,
		query,
		dst,
	)
}

func sendRequest(
	ctx context.Context,
	logger *slog.Logger,
	apiToken string,
	baseURL string,
	endpoint string,
	query string,
	dst any,
) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s", baseURL, endpoint))
	if err != nil {
		return err
	}

	u.RawQuery = query
	tempQuery := u.Query()
	tempQuery.Add("key", apiToken)
	u.RawQuery = tempQuery.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		logger.DebugContext(ctx, "hit 429, sleeping 1m")
		time.Sleep(time.Minute)
		return sendRequest(ctx, logger, apiToken, baseURL, endpoint, query, dst)
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

	err = httptools.ReadJSON(res.Body, dst)
	if err != nil {
		return err
	}

	return nil
}
