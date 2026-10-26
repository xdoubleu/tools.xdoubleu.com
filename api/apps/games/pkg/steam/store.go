package steam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"tools.xdoubleu.com/internal/httpclient"
)

// StoreURL is the public Steam store.
const StoreURL = "https://store.steampowered.com"

// StoreItem is a Steam store search hit.
type StoreItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	ImageURL string `json:"tiny_image"`
}

// StoreSearcher reads the public Steam store catalog; it needs no key.
type StoreSearcher interface {
	SearchStore(ctx context.Context, term string) ([]StoreItem, error)
	// ReviewCount returns how many reviews appID has, in every language.
	ReviewCount(ctx context.Context, appID int) (int, error)
}

const storeTimeout = 15 * time.Second

type storeClient struct {
	baseURL string
	http    *httpclient.Client
}

// NewStore reads the store at baseURL (StoreURL in production), trying each
// request once.
func NewStore(baseURL string) StoreSearcher {
	return storeClient{
		baseURL: baseURL,
		http: httpclient.New(httpclient.Config{
			Name:        "steam-store",
			Timeout:     storeTimeout,
			MaxAttempts: 1,
		}),
	}
}

// SearchStore returns the apps (not bundles or packages) matching term.
func (c storeClient) SearchStore(
	ctx context.Context,
	term string,
) ([]StoreItem, error) {
	q := url.Values{"term": {term}, "l": {"english"}, "cc": {"BE"}}
	var body struct {
		Items []StoreItem `json:"items"`
	}
	if err := c.get(ctx, "/api/storesearch/?"+q.Encode(), &body); err != nil {
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

func (c storeClient) ReviewCount(ctx context.Context, appID int) (int, error) {
	// The summary comes with the first page; one review is the smallest page.
	q := url.Values{
		"json": {"1"}, "language": {"all"}, "purchase_type": {"all"},
		"num_per_page": {"1"},
	}
	var body struct {
		Success      int `json:"success"`
		QuerySummary struct {
			TotalReviews int `json:"total_reviews"`
		} `json:"query_summary"`
	}
	path := "/appreviews/" + strconv.Itoa(appID) + "?" + q.Encode()
	if err := c.get(ctx, path, &body); err != nil {
		return 0, fmt.Errorf("steam reviews: %w", err)
	}
	if body.Success != 1 {
		return 0, fmt.Errorf("steam reviews: success %d", body.Success)
	}
	return body.QuerySummary.TotalReviews, nil
}

func (c storeClient) get(ctx context.Context, path string, dst any) error {
	resp, err := c.http.Do(ctx, httpclient.NewRequest(http.MethodGet, c.baseURL+path))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(bytes.NewReader(resp.Body)).Decode(dst)
}
