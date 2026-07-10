package openlibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// ErrNotFound is returned by GetByISBN when no book matches the given ISBN.
var ErrNotFound = errors.New("openlibrary: book not found")

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://openlibrary.org"

// backoffBase is the base delay for exponential backoff on retryable errors.
// Overridable in tests to keep them fast.
//
//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

const apiTimeout = 15 * time.Second

const (
	isbn13Len = 13
	// searchLimit caps the number of search results requested from Open Library.
	searchLimit = 20
	// searchFields whitelists the document fields Open Library returns, keeping
	// the search response small.
	searchFields = "key,title,author_name,cover_i,isbn,number_of_pages_median"

	// requestsPerSecond and burst control the shared token-bucket rate limiter.
	// Open Library does not publish an official rate limit; 10 req/s with a burst
	// of 10 is conservative enough to avoid 429s in practice.
	requestsPerSecond = 10
	burst             = 10

	// maxAttempts is the total number of tries for a retryable request (initial
	// attempt + retries).
	maxAttempts = 4

	// backoffCap is the maximum single sleep between retries.
	backoffCap = 30 * time.Second
)

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	limiter    *rate.Limiter
}

func New(logger *slog.Logger) Client {
	return client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
		limiter: rate.NewLimiter(requestsPerSecond, burst),
	}
}

func (c client) Search(ctx context.Context, query string) ([]ExternalBook, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(searchLimit))
	params.Set("fields", searchFields)
	endpoint := baseURL + "/search.json?" + params.Encode()

	var resp searchResponse
	if err := c.get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	books := make([]ExternalBook, 0, len(resp.Docs))
	for _, doc := range resp.Docs {
		books = append(books, docToExternalBook(doc))
	}

	return books, nil
}

// Get fetches a single work by its Open Library work ID via the search
// endpoint (key:/works/{id}), which — unlike the raw work record — resolves
// author names inline. The description is filled in separately since search
// results never carry one.
func (c client) Get(ctx context.Context, providerID string) (*ExternalBook, error) {
	params := url.Values{}
	params.Set("q", "key:/works/"+providerID)
	params.Set("limit", "1")
	params.Set("fields", searchFields)
	endpoint := baseURL + "/search.json?" + params.Encode()

	var resp searchResponse
	if err := c.get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	if len(resp.Docs) == 0 {
		return nil, ErrNotFound
	}

	book := docToExternalBook(resp.Docs[0])
	book.Description = c.fetchWorkDescription(ctx, "/works/"+providerID)

	return &book, nil
}

func (c client) GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error) {
	bibkey := "ISBN:" + isbn
	params := url.Values{}
	params.Set("bibkeys", bibkey)
	params.Set("format", "json")
	params.Set("jscmd", "details")
	endpoint := baseURL + "/api/books?" + params.Encode()

	var resp map[string]booksDetailsEntry
	if err := c.get(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	entry, ok := resp[bibkey]
	if !ok {
		return nil, ErrNotFound
	}

	book := detailsToExternalBook(isbn, entry.Details)

	// Descriptions live on the Work record, not on the edition returned by
	// jscmd=details. If the edition had no description and a work key is
	// available, fetch the work to fill it in.
	if book.Description == nil && len(entry.Details.Works) > 0 {
		book.Description = c.fetchWorkDescription(ctx, entry.Details.Works[0].Key)
	}

	return &book, nil
}

// fetchWorkDescription fetches the description field from an Open Library
// Work record (GET /works/OL…W.json). It returns nil on any error or when
// the work carries no description — the caller must treat a nil return as
// "no description available" rather than a hard failure.
func (c client) fetchWorkDescription(ctx context.Context, workKey string) *string {
	endpoint := baseURL + workKey + ".json"

	var work workResponse
	if err := c.get(ctx, endpoint, &work); err != nil {
		c.logger.WarnContext(ctx, "failed to fetch work description",
			slog.String("workKey", workKey),
			slog.Any("error", err),
		)
		return nil
	}

	if work.Description.Value == "" {
		return nil
	}

	v := work.Description.Value
	return &v
}

// FetchCover downloads the raw image bytes for the given Open Library cover
// URL. It appends ?default=false so that Open Library returns HTTP 404 instead
// of a blank placeholder image when no cover exists.
func (c client) FetchCover(
	ctx context.Context,
	coverURL string,
) ([]byte, string, error) {
	// Append default=false to get a proper 404 instead of a blank placeholder.
	endpoint := coverURL
	if strings.Contains(coverURL, "?") {
		endpoint += "&default=false"
	} else {
		endpoint += "?default=false"
	}

	var data []byte
	var contentType string

	err := c.doWithRetry(ctx, func() (bool, error) {
		if waitErr := c.limiter.Wait(ctx); waitErr != nil {
			return false, waitErr
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return false, reqErr
		}

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return false, ErrCoverNotFound
		}

		if isRetryableStatus(resp.StatusCode) {
			return true, fmt.Errorf(
				"openlibrary cover fetch returned %d for %s",
				resp.StatusCode,
				coverURL,
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			return false, fmt.Errorf(
				"openlibrary cover fetch returned %d for %s",
				resp.StatusCode,
				coverURL,
			)
		}

		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return false, fmt.Errorf("read cover body: %w", readErr)
		}

		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = "image/jpeg"
		}

		data = body
		contentType = ct
		return false, nil
	})

	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
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
				"openlibrary API returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"openlibrary API returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		return false, json.NewDecoder(resp.Body).Decode(dst)
	})
}

// doWithRetry calls attempt up to maxAttempts times, sleeping with exponential
// backoff between retries. attempt returns (retryable, error); a nil error
// stops immediately. Context cancellation is always non-retryable.
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

		// Never retry context cancellation — the caller has given up.
		if errors.Is(err, context.Canceled) {
			return err
		}

		lastErr = err

		if !retryable || i == maxAttempts-1 {
			break
		}

		delay := backoffDelay(i)
		c.logger.DebugContext(ctx, "retrying openlibrary request",
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

// backoffDelay returns the sleep duration for the given zero-based attempt
// index: 500ms, 1s, 2s, …, capped at backoffCap.
func backoffDelay(attempt int) time.Duration {
	d := backoffBase * (1 << attempt)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

// isRetryableStatus reports whether the HTTP status code warrants a retry
// (429 Too Many Requests or any 5xx server error).
func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		(status >= http.StatusInternalServerError &&
			status < 600)
}

// isTransientErr reports whether a transport error is likely transient and
// worth retrying. A deadline-exceeded error caused by the http.Client's own
// Timeout (not by the caller's context) is retryable; a caller cancellation
// is not.
func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	// DeadlineExceeded from the http.Client timeout is retryable.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// url.Error wraps the actual error; check IsTimeout for net-level timeouts.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Timeout()
	}
	return false
}

func docToExternalBook(doc searchDoc) ExternalBook {
	authors := make([]string, 0, len(doc.AuthorName))
	for _, name := range doc.AuthorName {
		if name != "" {
			authors = append(authors, name)
		}
	}

	isbn13 := pickISBN13(doc.ISBN)

	var coverURL *string
	if doc.CoverID != nil {
		u := CoverURLByID(*doc.CoverID)
		coverURL = &u
	} else if fallback := CoverURLByISBN(isbn13); fallback != "" {
		coverURL = &fallback
	}

	return ExternalBook{
		Provider:    "openlibrary",
		ProviderID:  strings.TrimPrefix(doc.Key, "/works/"),
		Title:       doc.Title,
		Authors:     authors,
		ISBN13:      isbn13,
		CoverURL:    coverURL,
		Description: nil,
		PageCount:   doc.NumberOfPagesMedian,
	}
}

func detailsToExternalBook(isbn string, d bookDetails) ExternalBook {
	authors := make([]string, 0, len(d.Authors))
	for _, a := range d.Authors {
		if a.Name != "" {
			authors = append(authors, a.Name)
		}
	}

	isbn13 := pickISBN13(d.ISBN13)
	if isbn13 == nil && len(isbn) == isbn13Len {
		v := isbn
		isbn13 = &v
	}

	var coverURL *string
	if len(d.Covers) > 0 {
		u := CoverURLByID(d.Covers[0])
		coverURL = &u
	} else if fallback := CoverURLByISBN(isbn13); fallback != "" {
		coverURL = &fallback
	}

	var desc *string
	if d.Description.Value != "" {
		v := d.Description.Value
		desc = &v
	}

	return ExternalBook{
		Provider:    "openlibrary",
		ProviderID:  "",
		Title:       d.Title,
		Authors:     authors,
		ISBN13:      isbn13,
		CoverURL:    coverURL,
		Description: desc,
		PageCount:   d.NumberOfPages,
	}
}

// pickISBN13 returns the first 13-digit ISBN found in the list, or nil.
func pickISBN13(isbns []string) *string {
	for _, raw := range isbns {
		v := raw
		if len(v) == isbn13Len {
			return &v
		}
	}
	return nil
}

// CoverURLByISBN returns an Open Library cover URL for the given ISBN13, or an
// empty string when no ISBN13 is available. Open Library serves covers keyed by
// ISBN without requiring an API key, so it is used as a fallback when no cover
// id is available.
func CoverURLByISBN(isbn13 *string) string {
	if isbn13 == nil || *isbn13 == "" {
		return ""
	}
	return "https://covers.openlibrary.org/b/isbn/" + *isbn13 + "-L.jpg"
}

// CoverURLByID returns an Open Library cover URL for the given numeric cover id.
func CoverURLByID(coverID int) string {
	return "https://covers.openlibrary.org/b/id/" + strconv.Itoa(coverID) + "-L.jpg"
}
