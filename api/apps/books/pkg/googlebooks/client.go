package googlebooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://www.googleapis.com/books/v1"

// backoffBase is the base delay for exponential backoff on retryable errors.
// backoffCap is the maximum single sleep between retries.
// Both are vars so tests can override them to keep suites fast.
//
//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 15 * time.Second

const (
	// searchLimit caps the number of results requested from the Google Books API.
	searchLimit = 5

	// requestsPerSecond and burst for the token-bucket rate limiter.
	// Without an API key Google Books allows ~1 req/s per IP; even with a key the
	// free tier limit is 1 000 req/day. Keep it conservative.
	requestsPerSecond = 1
	burst             = 3

	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4

	isbn13Type = "ISBN_13"

	// volumeInfoFields is the fields mask used in both Search and GetByISBN to
	// limit the response payload to only what ExternalBook needs.
	volumeInfoFields = "items(volumeInfo(" +
		"title,authors,description,pageCount,imageLinks,industryIdentifiers))"
)

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	limiter    *rate.Limiter
	apiKey     string
}

// New creates a Google Books client. apiKey is optional — leave empty to use
// the unauthenticated tier (lower rate limit). Set GOOGLE_BOOKS_API_KEY in
// production to raise the quota.
func New(logger *slog.Logger, apiKey string) Client {
	return client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
		limiter: rate.NewLimiter(requestsPerSecond, burst),
		apiKey:  apiKey,
	}
}

// Search queries the Google Books API for volumes matching query.
func (c client) Search(
	ctx context.Context,
	query string,
) ([]ExternalBook, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("maxResults", fmt.Sprintf("%d", searchLimit))
	params.Set("printType", "books")
	params.Set("fields", volumeInfoFields)
	c.addKey(params)
	endpoint := baseURL + "/volumes?" + params.Encode()

	var resp volumesResponse
	if err := c.get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	books := make([]ExternalBook, 0, len(resp.Items))
	for _, item := range resp.Items {
		books = append(books, volumeToExternalBook(item.VolumeInfo))
	}
	return books, nil
}

// GetByISBN returns the best-matching volume for the given ISBN-13 (or ISBN-10).
// Returns ErrNotFound when Google Books has no entry.
func (c client) GetByISBN(
	ctx context.Context,
	isbn string,
) (*ExternalBook, error) {
	params := url.Values{}
	params.Set("q", "isbn:"+isbn)
	params.Set("maxResults", "1")
	params.Set("printType", "books")
	params.Set("fields", volumeInfoFields)
	c.addKey(params)
	endpoint := baseURL + "/volumes?" + params.Encode()

	var resp volumesResponse
	if err := c.get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if len(resp.Items) == 0 {
		return nil, ErrNotFound
	}

	book := volumeToExternalBook(resp.Items[0].VolumeInfo)
	return &book, nil
}

// addKey appends the API key query parameter when one is configured.
func (c client) addKey(params url.Values) {
	if c.apiKey != "" {
		params.Set("key", c.apiKey)
	}
}

func volumeToExternalBook(vi volumeInfo) ExternalBook {
	var isbn13 *string
	for _, id := range vi.IndustryIdentifiers {
		if id.Type == isbn13Type && isbn13 == nil {
			v := id.Identifier
			isbn13 = &v
		}
	}

	var coverURL *string
	if vi.ImageLinks != nil && vi.ImageLinks.Thumbnail != "" {
		// Force HTTPS — Google Books sometimes returns http:// thumbnails.
		thumb := strings.Replace(
			vi.ImageLinks.Thumbnail, "http://", "https://", 1,
		)
		coverURL = &thumb
	}

	var desc *string
	if vi.Description != "" {
		v := vi.Description
		desc = &v
	}

	var pageCount *int
	if vi.PageCount > 0 {
		v := vi.PageCount
		pageCount = &v
	}

	authors := make([]string, 0, len(vi.Authors))
	for _, a := range vi.Authors {
		if a != "" {
			authors = append(authors, a)
		}
	}

	return ExternalBook{
		Title:       vi.Title,
		Authors:     authors,
		ISBN13:      isbn13,
		CoverURL:    coverURL,
		Description: desc,
		PageCount:   pageCount,
	}
}

func (c client) get(ctx context.Context, endpoint string, dst any) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		if waitErr := c.limiter.Wait(ctx); waitErr != nil {
			return false, waitErr
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Accept", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, fmt.Errorf(
				"googlebooks API returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"googlebooks API returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		return false, json.NewDecoder(resp.Body).Decode(dst)
	})
}

// doWithRetry calls attempt up to maxAttempts times with exponential backoff.
func (c client) doWithRetry(
	ctx context.Context,
	attempt func() (retryable bool, err error),
) error {
	var lastErr error
	for i := range maxAttempts {
		retryable, err := attempt()
		if err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) {
			return err
		}

		lastErr = err

		if !retryable || i == maxAttempts-1 {
			break
		}

		delay := backoffDelay(i)
		c.logger.DebugContext(ctx, "retrying googlebooks request",
			slog.Int("attempt", i+1),
			slog.Duration("backoff", delay),
			slog.Any("error", err),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// SetBaseURL overrides the Google Books API base URL. Intended for tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the exponential-backoff base delay. Intended for
// tests only so that retry tests run without real wall-clock sleeps.
func SetBackoffBase(d time.Duration) { backoffBase = d }

func backoffDelay(attempt int) time.Duration {
	d := backoffBase * (1 << attempt)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		(status >= http.StatusInternalServerError && status < 600)
}

func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Timeout()
	}
	return false
}
