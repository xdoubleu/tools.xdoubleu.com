// Package itunes is a minimal client for the iTunes Search and Lookup APIs,
// restricted to podcasts.
package itunes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://itunes.apple.com"

const (
	apiTimeout  = 5 * time.Second
	searchLimit = "25"
	podcast     = "podcast"
	// maxBody caps how much of a response is read.
	maxBody = 4 << 20
)

// ErrNotFound is returned by Lookup for an unknown or feedless show.
var ErrNotFound = errors.New("show not found")

// Show is a podcast with a feed to poll.
type Show struct {
	ID         int64
	Title      string
	Author     string
	ArtworkURL string
	FeedURL    string
	AppleURL   string
}

// Client is the subset of iTunes used for favourites.
type Client interface {
	// Search returns podcasts matching query; shows without a feed are dropped.
	Search(ctx context.Context, query string) ([]Show, error)
	// Lookup returns ErrNotFound for an unknown ID or a show without a feed.
	Lookup(ctx context.Context, id int64) (*Show, error)
}

type client struct {
	httpClient *http.Client
}

// New creates a client for the public iTunes API; it needs no key.
func New() Client {
	return client{httpClient: &http.Client{Timeout: apiTimeout}}
}

type response struct {
	Results []result `json:"results"`
}

type result struct {
	CollectionID   int64  `json:"collectionId"`
	CollectionName string `json:"collectionName"`
	ArtistName     string `json:"artistName"`
	FeedURL        string `json:"feedUrl"`
	ArtworkURL     string `json:"artworkUrl600"`
	ViewURL        string `json:"collectionViewUrl"`
}

func (r result) show() (Show, bool) {
	if r.CollectionID == 0 || r.FeedURL == "" {
		return Show{
			ID: 0, Title: "", Author: "", ArtworkURL: "", FeedURL: "", AppleURL: "",
		}, false
	}
	return Show{
		ID:         r.CollectionID,
		Title:      r.CollectionName,
		Author:     r.ArtistName,
		ArtworkURL: r.ArtworkURL,
		FeedURL:    r.FeedURL,
		AppleURL:   r.ViewURL,
	}, true
}

func (c client) Search(ctx context.Context, query string) ([]Show, error) {
	resp, err := c.get(ctx, "/search", url.Values{
		"term":   {query},
		"media":  {podcast},
		"entity": {podcast},
		"limit":  {searchLimit},
	})
	if err != nil {
		return nil, err
	}
	out := make([]Show, 0, len(resp.Results))
	for _, r := range resp.Results {
		if s, ok := r.show(); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (c client) Lookup(ctx context.Context, id int64) (*Show, error) {
	resp, err := c.get(ctx, "/lookup", url.Values{
		"id":     {strconv.FormatInt(id, 10)},
		"entity": {podcast},
	})
	if err != nil {
		return nil, err
	}
	for _, r := range resp.Results {
		if s, ok := r.show(); ok && s.ID == id {
			return &s, nil
		}
	}
	return nil, ErrNotFound
}

func (c client) get(
	ctx context.Context,
	path string,
	query url.Values,
) (*response, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, baseURL+path+"?"+query.Encode(), nil,
	)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("itunes %s: status %d", path, res.StatusCode)
	}
	var out response
	if err = json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&out); err != nil {
		return nil, fmt.Errorf("itunes %s: %w", path, err)
	}
	return &out, nil
}

// SetBaseURL overrides the API root. Tests only.
func SetBaseURL(u string) { baseURL = u }
