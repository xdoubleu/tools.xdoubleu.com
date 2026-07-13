package hardcover_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
)

const realBaseURL = "https://api.hardcover.app/v1/graphql"

func TestMain(m *testing.M) {
	// Speed up retries in all tests in this package.
	hardcover.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

// buildServer starts an httptest.Server that serves handler and overrides the
// package-level baseURL to point at it. The returned func closes the server and
// restores the original baseURL.
func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	hardcover.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		hardcover.SetBaseURL(realBaseURL)
	}
}

func TestGetByISBN_Found(t *testing.T) {
	body := isbnResponse(
		t,
		&editionFixture{ //nolint:exhaustruct // only fields under test
			Title:       "The Odyssey",
			Pages:       541,
			ISBN13:      "9780140447934",
			Cover:       "https://hardcover.app/edition-cover.jpg",
			BookDesc:    "An epic poem.",
			Authors:     []string{"Homer"},
			BookPages:   500,
			BookCover:   "https://hardcover.app/book-cover.jpg",
			HasBook:     true,
			FlatAuthors: false,
		},
	)

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780140447934")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "The Odyssey", got.Title)
	assert.Equal(t, []string{"Homer"}, got.Authors)
	require.NotNil(t, got.ISBN13)
	assert.Equal(t, "9780140447934", *got.ISBN13)
	require.NotNil(t, got.Description)
	assert.Equal(t, "An epic poem.", *got.Description)
	require.NotNil(t, got.PageCount)
	// Edition pages win over the book's page count.
	assert.Equal(t, 541, *got.PageCount)
	require.NotNil(t, got.CoverURL)
	// Edition cover wins over the book's cover.
	assert.Equal(t, "https://hardcover.app/edition-cover.jpg", *got.CoverURL)
}

// TestGetByISBN_FallsBackToBook verifies that when the edition omits title,
// pages and cover, the parent book's values fill the gaps.
func TestGetByISBN_FallsBackToBook(t *testing.T) {
	body := isbnResponse(t, &editionFixture{
		Title:       "",
		Pages:       0,
		ISBN13:      "9780140447934",
		Cover:       "",
		BookTitle:   "The Odyssey",
		BookDesc:    "An epic poem.",
		Authors:     []string{"Homer", "Emily Wilson"},
		BookPages:   500,
		BookCover:   "https://hardcover.app/book-cover.jpg",
		HasBook:     true,
		FlatAuthors: false,
	})

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780140447934")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "The Odyssey", got.Title)
	assert.Equal(t, []string{"Homer", "Emily Wilson"}, got.Authors)
	require.NotNil(t, got.PageCount)
	assert.Equal(t, 500, *got.PageCount)
	require.NotNil(t, got.CoverURL)
	assert.Equal(t, "https://hardcover.app/book-cover.jpg", *got.CoverURL)
}

// TestGetByISBN_FlatContributorName covers the flat "name" fallback used when
// cached_contributors is not nested under an author object.
func TestGetByISBN_FlatContributorName(t *testing.T) {
	body := isbnResponse(
		t,
		&editionFixture{ //nolint:exhaustruct // only fields under test
			Title:       "The Odyssey",
			ISBN13:      "9780140447934",
			Authors:     []string{"Homer"},
			HasBook:     true,
			FlatAuthors: true,
		},
	)

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780140447934")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"Homer"}, got.Authors)
}

func TestGetByISBN_NotFound(t *testing.T) {
	cleanup := buildServer(jsonHandler(json.RawMessage(`{"data":{"editions":[]}}`)))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780000000000")
	require.ErrorIs(t, err, hardcover.ErrNotFound)
	assert.Nil(t, got)
}

func TestGetByISBN_GraphQLError(t *testing.T) {
	body := `{"errors":[{"message":"field \"nope\" not found"}]}`
	cleanup := buildServer(jsonHandler(json.RawMessage(body)))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(context.Background(), "9780000000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GraphQL error")
}

func TestGetByISBN_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				isbnResponse(t, &editionFixture{ //nolint:exhaustruct // partial
					Title:   "Retry Book",
					ISBN13:  "9780000000001",
					Authors: []string{"Author"},
					HasBook: true,
				}),
			)
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780000000001")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Retry Book", got.Title)
	assert.GreaterOrEqual(t, attempts, 3, "expected at least 3 attempts")
}

func TestGetByISBN_TooManyRequests_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				isbnResponse(t, &editionFixture{ //nolint:exhaustruct // partial
					Title:   "Rate Book",
					ISBN13:  "9780000000002",
					Authors: []string{"Author"},
					HasBook: true,
				}),
			)
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780000000002")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.GreaterOrEqual(t, attempts, 2)
}

func TestSearch_ReturnsResults(t *testing.T) {
	books := searchResponse(t, []bookFixture{
		//nolint:exhaustruct // only fields under test
		{Title: "Space Odyssey", Authors: []string{"Clarke"}, Pages: 221},
		//nolint:exhaustruct // only fields under test
		{Title: "Another Book", Authors: []string{"Smith"}},
	})

	cleanup := buildServer(searchIDsThenBooksHandler(t, []int{1, 2}, books))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	results, err := c.Search(
		context.Background(),
		`intitle:"Space Odyssey" inauthor:"Clarke"`,
	)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "Space Odyssey", results[0].Title)
	assert.Equal(t, []string{"Clarke"}, results[0].Authors)
	require.NotNil(t, results[0].PageCount)
	assert.Equal(t, 221, *results[0].PageCount)
}

// TestSearch_SendsPlainQuery verifies the extracted title is sent as-is (no
// ILIKE wildcards) as the Typesense search query variable.
func TestSearch_SendsPlainQuery(t *testing.T) {
	var captured struct {
		Variables map[string]any `json:"variables"`
	}
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&captured)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(json.RawMessage(`{"data":{"search":{"ids":[]}}}`))
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.Search(context.Background(), `intitle:"Dune" inauthor:"Herbert"`)
	require.NoError(t, err)
	assert.Equal(t, "Dune", captured.Variables["query"])
}

func TestSearch_NoTitle_SkipsRequest(t *testing.T) {
	called := false
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(json.RawMessage(`{"data":{"search":{"ids":[]}}}`))
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	results, err := c.Search(context.Background(), "no-title-token")
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.False(t, called, "search without a title must not hit the API")
}

// TestSearch_NoIDs_SkipsSecondRequest verifies that when the Typesense search
// returns no IDs, Search returns early without querying books by ID.
func TestSearch_NoIDs_SkipsSecondRequest(t *testing.T) {
	requests := 0
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(json.RawMessage(`{"data":{"search":{"ids":[]}}}`))
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	results, err := c.Search(context.Background(), `intitle:"Dune"`)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, 1, requests, "no ids means the books-by-id request must be skipped")
}

// TestSearch_403_PropagatesError guards the reported regression: Hardcover's
// server rejects ilike/like/similar/regex operators with a 403, and that
// error must bubble up rather than being swallowed.
func TestSearch_403_PropagatesError(t *testing.T) {
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(
				`{"error":"ilike and related operations are not permitted on this server."}`,
			))
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.Search(context.Background(), `intitle:"Dune"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestGetByISBN_SendsBearerToken(t *testing.T) {
	var authHeader string
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(json.RawMessage(`{"data":{"editions":[]}}`))
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "secret-token")
	_, _ = c.GetByISBN(context.Background(), "9780000099999")
	assert.Equal(t, "Bearer secret-token", authHeader)
}

func TestGetByISBN_NonRetryable4xx(t *testing.T) {
	attempts := 0
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
		}),
	)
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(context.Background(), "9780000000000")
	require.Error(t, err)
	assert.Equal(t, 1, attempts, "4xx must not trigger retries")
}

func TestGetByISBN_ContextCanceled(t *testing.T) {
	cleanup := buildServer(jsonHandler(json.RawMessage(`{"data":{"editions":[]}}`)))
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before making any request

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(ctx, "9780000000000")
	require.ErrorIs(t, err, context.Canceled)
}

func TestGetByISBN_NetworkError(t *testing.T) {
	// Point to a port where nothing listens — httpClient.Do fails with a
	// *url.Error (connection refused), covering the isTransientErr url.Error path.
	hardcover.SetBaseURL("http://127.0.0.1:1")
	defer hardcover.SetBaseURL(realBaseURL)

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(context.Background(), "9780000000000")
	require.Error(t, err)
}

// --- helpers ---

type editionFixture struct {
	Title       string
	Pages       int
	ISBN13      string
	Cover       string
	BookTitle   string
	BookDesc    string
	BookPages   int
	BookCover   string
	Authors     []string
	HasBook     bool
	FlatAuthors bool
}

type bookFixture struct {
	Title   string
	Authors []string
	Desc    string
	Pages   int
	Cover   string
}

func contributorsJSON(authors []string, flat bool) []map[string]any {
	out := make([]map[string]any, 0, len(authors))
	for _, a := range authors {
		if flat {
			out = append(out, map[string]any{"name": a})
		} else {
			out = append(out, map[string]any{"author": map[string]any{"name": a}})
		}
	}
	return out
}

func bookJSON(f bookFixture) map[string]any {
	b := map[string]any{
		"title":               f.Title,
		"description":         f.Desc,
		"pages":               f.Pages,
		"cached_contributors": contributorsJSON(f.Authors, false),
	}
	if f.Cover != "" {
		b["cached_image"] = map[string]any{"url": f.Cover}
	}
	return b
}

func isbnResponse(t *testing.T, f *editionFixture) json.RawMessage {
	t.Helper()

	ed := map[string]any{
		"title":   f.Title,
		"pages":   f.Pages,
		"isbn_13": f.ISBN13,
	}
	if f.Cover != "" {
		ed["image"] = map[string]any{"url": f.Cover}
	}
	if f.HasBook {
		ed["book"] = map[string]any{
			"title":               f.BookTitle,
			"description":         f.BookDesc,
			"pages":               f.BookPages,
			"cached_contributors": contributorsJSON(f.Authors, f.FlatAuthors),
			"cached_image":        map[string]any{"url": f.BookCover},
		}
	}

	resp := map[string]any{
		"data": map[string]any{"editions": []map[string]any{ed}},
	}
	return mustJSON(t, resp)
}

func searchResponse(t *testing.T, books []bookFixture) json.RawMessage {
	t.Helper()

	out := make([]map[string]any, 0, len(books))
	for _, f := range books {
		out = append(out, bookJSON(f))
	}
	resp := map[string]any{"data": map[string]any{"books": out}}
	return mustJSON(t, resp)
}

// searchIDsThenBooksHandler serves the two-request Search flow: the first
// POST (search by title) gets ids, the second POST (books by id) gets booksBody.
func searchIDsThenBooksHandler(
	t *testing.T,
	ids []int,
	booksBody json.RawMessage,
) http.Handler {
	t.Helper()

	first := true
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if first {
			first = false
			idsResp := map[string]any{
				"data": map[string]any{"search": map[string]any{"ids": ids}},
			}
			_, _ = w.Write(mustJSON(t, idsResp))
			return
		}
		_, _ = w.Write(booksBody)
	})
}

func jsonHandler(body json.RawMessage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
