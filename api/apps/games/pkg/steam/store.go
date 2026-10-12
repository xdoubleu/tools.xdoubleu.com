package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// StoreItem is a Steam store search hit.
type StoreItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	ImageURL string `json:"tiny_image"`
}

// StoreSearcher searches the public Steam store catalog; it needs no key.
type StoreSearcher interface {
	SearchStore(ctx context.Context, term string) ([]StoreItem, error)
}

const storeTimeout = 15 * time.Second

type storeClient struct {
	baseURL string
	http    *http.Client
}

// NewStore searches the store at baseURL (StorefrontURLRESTAPI in production).
func NewStore(baseURL string) StoreSearcher {
	return storeClient{baseURL: baseURL, http: &http.Client{Timeout: storeTimeout}}
}

// SearchStore returns the apps (not bundles or packages) matching term.
func (c storeClient) SearchStore(
	ctx context.Context,
	term string,
) ([]StoreItem, error) {
	q := url.Values{"term": {term}, "l": {"english"}, "cc": {"BE"}}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL+"/storesearch/?"+q.Encode(), nil,
	)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam store search: status %d", resp.StatusCode)
	}
	var body struct {
		Items []StoreItem `json:"items"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("steam store search: %w", err)
	}
	apps := body.Items[:0]
	for _, it := range body.Items {
		if it.Type == "app" {
			apps = append(apps, it)
		}
	}
	return apps, nil
}
