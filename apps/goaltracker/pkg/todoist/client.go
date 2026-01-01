package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	httptools "github.com/XDoubleU/essentia/pkg/communication/http"
)

const BaseURLRESTAPI = "https://api.todoist.com/rest/v2"

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

	err = httptools.ReadJSON(res.Body, dst)
	if err != nil && err.Error() != "body must not be empty" {
		return err
	}

	return nil
}
