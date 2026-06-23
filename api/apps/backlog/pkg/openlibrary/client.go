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
)

// ErrNotFound is returned by GetByISBN when no book matches the given ISBN.
var ErrNotFound = errors.New("openlibrary: book not found")

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://openlibrary.org"

const apiTimeout = 5 * time.Second

const (
	isbn13Len = 13
	isbn10Len = 10
	// searchLimit caps the number of search results requested from Open Library.
	searchLimit = 20
	// searchFields whitelists the document fields Open Library returns, keeping
	// the search response small.
	searchFields = "key,title,author_name,cover_i,isbn,number_of_pages_median"
)

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
}

func New(logger *slog.Logger) Client {
	return client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
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
	return &book, nil
}

func (c client) get(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"openlibrary API returned %d: %s",
			resp.StatusCode,
			string(raw),
		)
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}

func docToExternalBook(doc searchDoc) ExternalBook {
	authors := make([]string, 0, len(doc.AuthorName))
	for _, name := range doc.AuthorName {
		if name != "" {
			authors = append(authors, name)
		}
	}

	isbn13, isbn10 := pickISBNs(doc.ISBN)

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
		ISBN10:      isbn10,
		CoverURL:    coverURL,
		Description: nil,
		PageCount:   doc.NumberOfPagesMedian,
	}
}

func detailsToExternalBook(isbn string, d bookDetails) ExternalBook {
	isbn13, isbn10 := pickISBNs(append(append([]string{}, d.ISBN13...), d.ISBN10...))
	if isbn13 == nil && len(isbn) == isbn13Len {
		v := isbn
		isbn13 = &v
	}
	if isbn10 == nil && len(isbn) == isbn10Len {
		v := isbn
		isbn10 = &v
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
		Authors:     nil,
		ISBN13:      isbn13,
		ISBN10:      isbn10,
		CoverURL:    coverURL,
		Description: desc,
		PageCount:   d.NumberOfPages,
	}
}

// pickISBNs returns the first 13- and 10-digit ISBNs found in the list.
func pickISBNs(isbns []string) (*string, *string) {
	var isbn13, isbn10 *string
	for _, raw := range isbns {
		v := raw
		switch len(v) {
		case isbn13Len:
			if isbn13 == nil {
				isbn13 = &v
			}
		case isbn10Len:
			if isbn10 == nil {
				isbn10 = &v
			}
		}
	}
	return isbn13, isbn10
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
