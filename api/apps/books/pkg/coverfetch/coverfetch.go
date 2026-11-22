// Package coverfetch downloads cover images from URLs that can be
// user-supplied, so every request goes through safedial.
package coverfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/safedial"
)

// fetchTimeout: the cover read path fetches inline in a public handler, so a
// dead source must fail fast rather than outlive the server's write timeout.
const fetchTimeout = 5 * time.Second

const maxRedirects = 5

// Client fetches cover images.
type Client struct {
	http *http.Client
}

// New returns a Client; private addresses are reachable only outside
// production.
func New(env string) *Client {
	return &Client{
		http: safedial.Client(fetchTimeout, maxRedirects, env != config.ProdEnv),
	}
}

// Fetch GETs url and returns its body, failing when the response is not 2xx
// or is larger than maxBytes.
func (c *Client) Fetch(ctx context.Context, url string, maxBytes int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build cover request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch cover: %s returned %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read cover body: %w", err)
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("cover from %s exceeds %d bytes", url, maxBytes)
	}
	return data, nil
}
