package hardcover

import (
	"bytes"
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
var baseURL = "https://api.hardcover.app/v1/graphql"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 30 * time.Second

const (
	// searchLimit caps the number of results requested from a title search.
	searchLimit = 5

	// requestsPerSecond and burst for the token-bucket rate limiter. Hardcover
	// allows 60 req/min with no daily cap; keep it conservative at ~1 req/s.
	requestsPerSecond = 1
	burst             = 3

	// maxAttempts is the total number of tries for a retryable request.
	maxAttempts = 4
)

// isbnQuery looks up a single edition by its ISBN-13 and pulls the parent
// book's denormalised metadata. Selection depth stays within Hardcover's max
// query depth of 3 (editions → image → url; editions → book → cached_image),
// so cached_image/cached_contributors are used instead of deep relation joins.
const isbnQuery = `query BookByISBN($isbn: String!) {
  editions(where: {isbn_13: {_eq: $isbn}}, limit: 1) {
    title
    pages
    isbn_13
    image { url }
    book {
      title
      pages
      description
      cached_image
      cached_contributors
    }
  }
}`

// searchIDsQuery finds book IDs via Hardcover's Typesense-backed search index
// (the same index the website uses). This is the only fuzzy-match path
// Hardcover permits: its Hasura server rejects ilike/like/similar/regex
// operators on the books table with a 403, so a title filter there cannot do
// fuzzy matching — only search() can.
const searchIDsQuery = `query SearchBookIDs($query: String!, $perPage: Int!) {
  search(query: $query, query_type: "Book", per_page: $perPage, page: 1) {
    ids
  }
}`

// booksByIDsQuery fetches full book records for IDs returned by
// searchIDsQuery. Uses _in, which (unlike ilike) is a permitted operator.
const booksByIDsQuery = `query BooksByIDs($ids: [Int!]!) {
  books(where: {id: {_in: $ids}}) {
    id
    title
    pages
    description
    cached_image
    cached_contributors
  }
}`

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	limiter    *rate.Limiter
	apiKey     string
}

// New creates a Hardcover client. apiKey is the Bearer JWT from the Hardcover
// account settings page; an empty key still constructs a client but every
// request will be rejected by Hardcover — callers should leave the client nil
// when no key is configured.
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

// GetByISBN returns the best-matching edition for the given ISBN-13.
// Returns ErrNotFound when Hardcover has no matching edition.
func (c client) GetByISBN(
	ctx context.Context,
	isbn string,
) (*ExternalBook, error) {
	var resp isbnResponse
	err := c.post(ctx, isbnQuery, map[string]any{"isbn": isbn}, &resp)
	if err != nil {
		return nil, err
	}
	if err = graphQLErr(resp.Errors); err != nil {
		return nil, err
	}
	if len(resp.Data.Editions) == 0 {
		return nil, ErrNotFound
	}

	out := editionToExternalBook(resp.Data.Editions[0])
	return &out, nil
}

// Search queries Hardcover for books matching the title in query. It first
// resolves matching book IDs via the Typesense search index, then fetches
// the full records for those IDs.
func (c client) Search(
	ctx context.Context,
	query string,
) ([]ExternalBook, error) {
	title := extractTitle(query)
	if title == "" {
		return nil, nil
	}

	var idsResp searchIDsResponse
	err := c.post(ctx, searchIDsQuery, map[string]any{
		"query":   title,
		"perPage": searchLimit,
	}, &idsResp)
	if err != nil {
		return nil, err
	}
	if err = graphQLErr(idsResp.Errors); err != nil {
		return nil, err
	}
	if len(idsResp.Data.Search.IDs) == 0 {
		return nil, nil
	}

	var resp searchResponse
	err = c.post(ctx, booksByIDsQuery, map[string]any{
		"ids": idsResp.Data.Search.IDs,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if err = graphQLErr(resp.Errors); err != nil {
		return nil, err
	}

	// booksByIDsQuery has no order_by, so Hasura/Postgres returns rows in its
	// own default order rather than the Typesense relevance order the IDs
	// arrived in. Reindex by ID and re-emit in that original order.
	byID := make(map[int]book, len(resp.Data.Books))
	for _, b := range resp.Data.Books {
		byID[b.ID] = b
	}
	books := make([]ExternalBook, 0, len(idsResp.Data.Search.IDs))
	for _, id := range idsResp.Data.Search.IDs {
		if b, ok := byID[id]; ok {
			books = append(books, bookToExternalBook(b))
		}
	}
	return books, nil
}

// editionToExternalBook merges an edition with its parent book, preferring
// edition-level values (title, pages, cover, ISBN) and filling gaps from the
// book (description, authors, and title/pages/cover when the edition omits
// them).
func editionToExternalBook(e edition) ExternalBook {
	var out ExternalBook
	if e.Book != nil {
		out = bookToExternalBook(*e.Book)
	}

	if e.Title != "" {
		out.Title = e.Title
	}
	if e.Pages > 0 {
		out.PageCount = &e.Pages
	}
	if e.Image != nil && e.Image.URL != "" {
		url := e.Image.URL
		out.CoverURL = &url
	}
	if e.ISBN13 != "" {
		isbn := e.ISBN13
		out.ISBN13 = &isbn
	}
	return out
}

// bookToExternalBook maps a work-level book record to an ExternalBook.
func bookToExternalBook(b book) ExternalBook {
	out := ExternalBook{ //nolint:exhaustruct // ISBN13 lives on editions only
		Title: b.Title,
	}

	authors := make([]string, 0, len(b.CachedContributor))
	for _, cc := range b.CachedContributor {
		if n := cc.name(); n != "" {
			authors = append(authors, n)
		}
	}
	out.Authors = authors

	if b.Description != "" {
		desc := b.Description
		out.Description = &desc
	}
	if b.Pages > 0 {
		pages := b.Pages
		out.PageCount = &pages
	}
	if b.CachedImage != nil && b.CachedImage.URL != "" {
		url := b.CachedImage.URL
		out.CoverURL = &url
	}
	return out
}

// graphQLErr collapses a GraphQL errors array into a single Go error.
func graphQLErr(errs []graphQLError) error {
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Message)
	}
	return fmt.Errorf("hardcover GraphQL error: %s", strings.Join(msgs, "; "))
}

// extractTitle pulls the title out of an intitle:"..." query token. Returns ""
// when no quoted title is present (caller should skip the search).
func extractTitle(query string) string {
	const prefix = `intitle:"`
	idx := strings.Index(query, prefix)
	if idx < 0 {
		return ""
	}
	rest := query[idx+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// post sends a GraphQL query and decodes the JSON response into dst. GraphQL
// errors are surfaced on dst's Errors field and checked by the caller after
// decoding.
func (c client) post(
	ctx context.Context,
	query string,
	variables map[string]any,
	dst any,
) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}

	return c.doWithRetry(ctx, func() (bool, error) {
		if waitErr := c.limiter.Wait(ctx); waitErr != nil {
			return false, waitErr
		}

		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodPost, baseURL, bytes.NewReader(body),
		)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, fmt.Errorf(
				"hardcover API returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"hardcover API returned %d: %s",
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
		c.logger.DebugContext(ctx, "retrying hardcover request",
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

// SetBaseURL overrides the Hardcover GraphQL endpoint. Intended for tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the exponential-backoff base delay. Intended for
// tests only so retry tests run without real wall-clock sleeps.
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
