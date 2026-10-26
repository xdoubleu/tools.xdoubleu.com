package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"tools.xdoubleu.com/internal/httpclient"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.hardcover.app/v1/graphql"

const apiTimeout = 30 * time.Second

const (
	// searchLimit: the query is title-only, so for a common title the right
	// author's book can rank deep; the caller's author filter needs the depth.
	searchLimit = 25

	// Hardcover allows 60 req/min with no daily cap; stay at ~1 req/s.
	requestsPerSecond = 1
	burst             = 3

	maxAttempts = 4
	// deadlineAttempts replaces maxAttempts when the caller's deadline is
	// shorter than maxAttempts full attempts (a request's is), so the retry
	// still fits.
	deadlineAttempts = 2
)

// isbnQuery stays within Hardcover's max query depth of 3, hence
// cached_image/cached_contributors instead of deep joins.
const isbnQuery = `query BookByISBN($isbn: String!) {
  editions(where: {isbn_13: {_eq: $isbn}}, limit: 1) {
    title
    pages
    isbn_13
    image { url }
    book {
      id
      title
      pages
      description
      cached_image
      cached_contributors
    }
  }
}`

// searchIDsQuery uses the Typesense search index, Hardcover's only fuzzy path:
// _like/_ilike/_similar/_regex are disabled API-wide, so author
// disambiguation happens post-fetch in the caller.
const searchIDsQuery = `query SearchBookIDs($query: String!, $perPage: Int!) {
  search(query: $query, query_type: "Book", per_page: $perPage, page: 1) {
    ids
  }
}`

// booksByIDsQuery fetches full records for searchIDsQuery's IDs.
// books→book_series→series is three levels, the depth limit.
const booksByIDsQuery = `query BooksByIDs($ids: [Int!]!) {
  books(where: {id: {_in: $ids}}) {
    id
    title
    pages
    description
    cached_image
    cached_contributors
    users_count
    editions(limit: 1) {
      isbn_13
    }
    book_series {
      position
      series { name primary_books_count }
    }
  }
}`

type client struct {
	logger *slog.Logger
	http   *httpclient.Client
	apiKey string
}

// New creates a Hardcover client. apiKey is the account's Bearer JWT; leave
// the client nil when no key is configured.
func New(logger *slog.Logger, apiKey string) Client {
	return client{
		logger: logger,
		http: httpclient.New(httpclient.Config{
			Name:             "hardcover",
			Logger:           logger,
			Timeout:          apiTimeout,
			MaxAttempts:      maxAttempts,
			DeadlineAttempts: deadlineAttempts,
			Limiter:          rate.NewLimiter(requestsPerSecond, burst),
		}),
		apiKey: apiKey,
	}
}

// GetByISBN returns the best edition for an ISBN-13, or ErrNotFound.
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

	ed := resp.Data.Editions[0]
	out := editionToExternalBook(ed)
	if ed.Book != nil && ed.Book.ID != 0 {
		out.Series = c.bookSeries(ctx, ed.Book.ID)
	}
	return &out, nil
}

// Search resolves book IDs via the Typesense index, then fetches the records.
func (c client) Search(
	ctx context.Context,
	query string,
) ([]ExternalBook, error) {
	terms := extractSearchTerms(query)
	if terms == "" {
		return nil, nil
	}

	var idsResp searchIDsResponse
	err := c.post(ctx, searchIDsQuery, map[string]any{
		"query":   terms,
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

	// No order_by, so restore the Typesense relevance order by ID.
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

// editionToExternalBook prefers edition values, filling gaps from the book.
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

// bookToExternalBook borrows one edition's ISBN13, since works carry none.
func bookToExternalBook(b book) ExternalBook {
	out := ExternalBook{} //nolint:exhaustruct // fields set below
	out.Title = b.Title

	authors := make([]string, 0, len(b.CachedContributor))
	for _, cc := range b.CachedContributor {
		if n := cc.name(); n != "" {
			authors = append(authors, n)
		}
	}
	out.Authors = authors

	if len(b.Editions) > 0 && b.Editions[0].ISBN13 != "" {
		isbn := b.Editions[0].ISBN13
		out.ISBN13 = &isbn
	}

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
	out.Series = pickSeries(b.BookSeries)
	out.Readers = b.UsersCount
	return out
}

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

// extractSearchTerms returns only the title from a buildSearchQuery-style
// string: Typesense weights titles highest, so adding the author surfaces
// companions titled after the author. "" means skip the search.
func extractSearchTerms(query string) string {
	return extractQuotedField(query, "intitle:\"")
}

// extractQuotedField returns the quoted value after prefix, or "".
func extractQuotedField(query, prefix string) string {
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

// post sends a GraphQL query; the caller checks dst's Errors.
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

	req := httpclient.NewRequest(http.MethodPost, baseURL).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body)
	if c.apiKey != "" {
		req.SetHeader("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(ctx, req)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"hardcover API returned %d: %s", resp.StatusCode, resp.Snippet(),
		)
	}

	return json.NewDecoder(bytes.NewReader(resp.Body)).Decode(dst)
}

// SetBaseURL overrides the Hardcover GraphQL endpoint. Tests only.
func SetBaseURL(u string) { baseURL = u }
