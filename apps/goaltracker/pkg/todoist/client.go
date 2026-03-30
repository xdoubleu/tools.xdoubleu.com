package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
)

const BaseURLRESTAPI = "https://api.todoist.com/api/v1"

type client struct {
	apiToken string
}

func New(apiToken string) Client {
	return client{
		apiToken: apiToken,
	}
}

func (client client) sendRequest(
	ctx context.Context,
	method string,
	endpoint string,
	query string,
	body any,
	dst any,
) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s", BaseURLRESTAPI, endpoint))
	if err != nil {
		return err
	}

	u.RawQuery = query

	var req *http.Request
	if body != nil {
		var marshalled []byte
		marshalled, err = json.Marshal(body)
		if err != nil {
			return err
		}

		req, err = http.NewRequestWithContext(
			ctx,
			method,
			u.String(),
			bytes.NewBuffer(marshalled),
		)
		if err != nil {
			return err
		}

		req.Header.Add("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return err
		}
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", client.apiToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

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
	if err != nil && err.Error() != "body must not be empty" {
		return err
	}

	return nil
}
