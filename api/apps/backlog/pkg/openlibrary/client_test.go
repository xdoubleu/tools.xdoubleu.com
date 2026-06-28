//nolint:testpackage //needs internal access to override baseURL for testing
package openlibrary

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

// TestMain overrides backoffBase so that retry-backoff tests complete in
// milliseconds instead of seconds. The rate limiter burst (10) covers all
// requests made in a single test without adding latency.
func TestMain(m *testing.M) {
	backoffBase = time.Millisecond
	os.Exit(m.Run())
}

// setupTestServer starts an httptest server that routes by path: /search.json
// returns searchPayload and /api/books returns booksPayload. Either payload may
// be nil to serve an empty JSON object. Unknown paths (e.g. /works/…) return
// an empty JSON object, which lets tests that don't need work payloads pass
// without modification.
func setupTestServer(t *testing.T, searchPayload, booksPayload any) {
	t.Helper()
	setupTestServerFull(t, searchPayload, booksPayload, nil)
}

// setupTestServerFull is like setupTestServer but also routes work record
// requests. workPayloads is keyed by the URL path (e.g. "/works/OL27448W.json");
// paths not found in the map return an empty JSON object.
func setupTestServerFull(
	t *testing.T,
	searchPayload, booksPayload any,
	workPayloads map[string]any,
) {
	t.Helper()
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			var payload any = map[string]any{}
			switch {
			case strings.HasPrefix(r.URL.Path, "/search.json"):
				if searchPayload != nil {
					payload = searchPayload
				}
			case strings.HasPrefix(r.URL.Path, "/api/books"):
				if booksPayload != nil {
					payload = booksPayload
				}
			default:
				if workPayloads != nil {
					if p, ok := workPayloads[r.URL.Path]; ok {
						payload = p
					}
				}
			}
			require.NoError(t, json.NewEncoder(w).Encode(payload))
		}),
	)
	t.Cleanup(func() {
		srv.Close()
		baseURL = "https://openlibrary.org"
	})
	baseURL = srv.URL
}

func TestSearch_ReturnsMappedBooks(t *testing.T) {
	setupTestServer(t, map[string]any{
		"docs": []map[string]any{
			{
				"key":                    "/works/OL27448W",
				"title":                  "The Lord of the Rings",
				"author_name":            []string{"J.R.R. Tolkien"},
				"cover_i":                258027,
				"isbn":                   []string{"059035342X", "9780618640157"},
				"number_of_pages_median": 1216,
			},
		},
	}, nil)

	c := New(logging.NewNopLogger())
	results, err := c.Search(context.Background(), "lord of the rings")
	require.NoError(t, err)
	require.Len(t, results, 1)

	book := results[0]
	assert.Equal(t, "openlibrary", book.Provider)
	assert.Equal(t, "OL27448W", book.ProviderID)
	assert.Equal(t, "The Lord of the Rings", book.Title)
	assert.Equal(t, []string{"J.R.R. Tolkien"}, book.Authors)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, "9780618640157", *book.ISBN13)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, CoverURLByID(258027), *book.CoverURL)
	require.NotNil(t, book.PageCount)
	assert.Equal(t, 1216, *book.PageCount)
	// Search results never carry a description.
	assert.Nil(t, book.Description)
}

func TestSearch_FallsBackToISBNCover(t *testing.T) {
	setupTestServer(t, map[string]any{
		"docs": []map[string]any{
			{
				"key":         "/works/OL1W",
				"title":       "No Cover Id",
				"author_name": []string{},
				"isbn":        []string{"9780618640157"},
			},
		},
	}, nil)

	c := New(logging.NewNopLogger())
	results, err := c.Search(context.Background(), "x")
	require.NoError(t, err)
	require.Len(t, results, 1)

	isbn13 := "9780618640157"
	require.NotNil(t, results[0].CoverURL)
	assert.Equal(t, CoverURLByISBN(&isbn13), *results[0].CoverURL)
}

func TestSearch_EmptyResults(t *testing.T) {
	setupTestServer(t, map[string]any{"docs": []any{}}, nil)

	c := New(logging.NewNopLogger())
	results, err := c.Search(context.Background(), "xyz")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}),
	)
	t.Cleanup(func() {
		srv.Close()
		baseURL = "https://openlibrary.org"
	})
	baseURL = srv.URL

	c := New(logging.NewNopLogger())
	_, err := c.Search(context.Background(), "anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGetByISBN_ReturnsMetadata(t *testing.T) {
	isbn := "9780618640157"
	setupTestServer(t, nil, map[string]any{
		"ISBN:" + isbn: map[string]any{
			"details": map[string]any{
				"title":           "The Lord of the Rings",
				"description":     "An epic high-fantasy novel.",
				"number_of_pages": 1216,
				"covers":          []int{258027},
				"isbn_13":         []string{isbn},
				"isbn_10":         []string{"0618640150"},
			},
		},
	})

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err)
	require.NotNil(t, book)
	assert.Equal(t, "openlibrary", book.Provider)
	assert.Equal(t, "The Lord of the Rings", book.Title)
	require.NotNil(t, book.Description)
	assert.Equal(t, "An epic high-fantasy novel.", *book.Description)
	require.NotNil(t, book.PageCount)
	assert.Equal(t, 1216, *book.PageCount)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, CoverURLByID(258027), *book.CoverURL)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, isbn, *book.ISBN13)
}

func TestGetByISBN_DescriptionObject(t *testing.T) {
	isbn := "9780618640157"
	setupTestServer(t, nil, map[string]any{
		"ISBN:" + isbn: map[string]any{
			"details": map[string]any{
				"title": "Object Description",
				"description": map[string]any{
					"type":  "/type/text",
					"value": "Wrapped description.",
				},
			},
		},
	})

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err)
	require.NotNil(t, book.Description)
	assert.Equal(t, "Wrapped description.", *book.Description)
	// No covers and no isbn_13 in payload: falls back to the queried ISBN.
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, isbn, *book.ISBN13)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, CoverURLByISBN(&isbn), *book.CoverURL)
}

func TestGetByISBN_NotFound(t *testing.T) {
	setupTestServer(t, nil, map[string]any{})

	c := New(logging.NewNopLogger())
	_, err := c.GetByISBN(context.Background(), "0000000000000")
	require.ErrorIs(t, err, ErrNotFound)
}

// TestGetByISBN_FetchesDescriptionFromWork verifies that when the edition
// record has no description but references a Work, GetByISBN fetches the Work
// record and returns its description.
func TestGetByISBN_FetchesDescriptionFromWork(t *testing.T) {
	isbn := "9780618640157"
	setupTestServerFull(t,
		nil,
		map[string]any{
			"ISBN:" + isbn: map[string]any{
				"details": map[string]any{
					"title":   "The Lord of the Rings",
					"covers":  []int{258027},
					"isbn_13": []string{isbn},
					// No description on the edition; work key is present.
					"works": []map[string]any{
						{"key": "/works/OL27448W"},
					},
				},
			},
		},
		map[string]any{
			"/works/OL27448W.json": map[string]any{
				"description": "An epic high-fantasy novel set in Middle-earth.",
			},
		},
	)

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err)
	require.NotNil(t, book)
	require.NotNil(t, book.Description)
	assert.Equal(
		t,
		"An epic high-fantasy novel set in Middle-earth.",
		*book.Description,
	)
}

// TestGetByISBN_WorkFetchError_GracefulFallback verifies that a failure to
// fetch the Work record does not cause GetByISBN to return an error — the book
// is returned with a nil description instead.
func TestGetByISBN_WorkFetchError_GracefulFallback(t *testing.T) {
	isbn := "9780618640157"

	// Serve /api/books normally; serve all work paths with a 500 to simulate
	// an Open Library outage on the work endpoint.
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/books") {
				w.Header().Set("Content-Type", "application/json")
				payload := map[string]any{
					"ISBN:" + isbn: map[string]any{
						"details": map[string]any{
							"title":   "Some Book",
							"isbn_13": []string{isbn},
							"works":   []map[string]any{{"key": "/works/OL1W"}},
						},
					},
				}
				require.NoError(t, json.NewEncoder(w).Encode(payload))
				return
			}
			http.Error(w, "server error", http.StatusInternalServerError)
		}),
	)
	t.Cleanup(func() {
		srv.Close()
		baseURL = "https://openlibrary.org"
	})
	baseURL = srv.URL

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err, "work fetch failure must not propagate as an error")
	require.NotNil(t, book)
	assert.Nil(t, book.Description)
}

func TestCoverURLByISBN(t *testing.T) {
	assert.Empty(t, CoverURLByISBN(nil))
	empty := ""
	assert.Empty(t, CoverURLByISBN(&empty))
	isbn := "9780618640157"
	assert.Equal(
		t,
		"https://covers.openlibrary.org/b/isbn/9780618640157-L.jpg",
		CoverURLByISBN(&isbn),
	)
}

func TestCoverURLByID(t *testing.T) {
	assert.Equal(
		t,
		"https://covers.openlibrary.org/b/id/258027-L.jpg",
		CoverURLByID(258027),
	)
}

// --- retry / backoff tests ---
// These rely on backoffBase being set to time.Millisecond by TestMain so they
// complete quickly without wall-clock delays.

// setupRetryServer creates an httptest server whose handler is fully
// controlled by the caller. The baseURL is pointed at the server and restored
// on cleanup.
func setupRetryServer(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		srv.Close()
		baseURL = "https://openlibrary.org"
	})
	baseURL = srv.URL
}

// validBooksPayload returns a well-formed /api/books JSON payload for isbn.
func validBooksPayload(isbn string) map[string]any {
	return map[string]any{
		"ISBN:" + isbn: map[string]any{
			"details": map[string]any{
				"title":   "Retry Book",
				"isbn_13": []string{isbn},
			},
		},
	}
}

// TestRetry_429ThenSuccess verifies that a single 429 response is retried and
// the eventual 200 is returned to the caller.
func TestRetry_429ThenSuccess(t *testing.T) {
	isbn := "9780618640157"
	var calls atomic.Int32
	payload := validBooksPayload(isbn)

	setupRetryServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if strings.HasPrefix(r.URL.Path, "/api/books") && n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(payload))
	})

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err)
	assert.Equal(t, "Retry Book", book.Title)
	assert.GreaterOrEqual(t, calls.Load(), int32(2), "expected at least one retry")
}

// TestRetry_5xxThenSuccess verifies that a 500 response is retried and the
// eventual 200 is returned.
func TestRetry_5xxThenSuccess(t *testing.T) {
	isbn := "9780618640157"
	var calls atomic.Int32
	payload := validBooksPayload(isbn)

	setupRetryServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if strings.HasPrefix(r.URL.Path, "/api/books") && n == 1 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(payload))
	})

	c := New(logging.NewNopLogger())
	book, err := c.GetByISBN(context.Background(), isbn)
	require.NoError(t, err)
	assert.Equal(t, "Retry Book", book.Title)
	assert.GreaterOrEqual(t, calls.Load(), int32(2), "expected at least one retry")
}

// TestRetry_NonRetryable4xx verifies that a 400 response is NOT retried — the
// error is returned immediately after a single attempt.
func TestRetry_NonRetryable4xx(t *testing.T) {
	var calls atomic.Int32

	setupRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	})

	c := New(logging.NewNopLogger())
	_, err := c.GetByISBN(context.Background(), "9780618640157")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Equal(t, int32(1), calls.Load(), "non-retryable 4xx must not be retried")
}

// TestRetry_ExhaustedAttempts verifies that after maxAttempts the last error
// is returned and no further requests are made.
func TestRetry_ExhaustedAttempts(t *testing.T) {
	var calls atomic.Int32

	setupRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := New(logging.NewNopLogger())
	_, err := c.GetByISBN(context.Background(), "9780618640157")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Equal(
		t, int32(maxAttempts), calls.Load(),
		"should make exactly maxAttempts requests before giving up",
	)
}

// netTimeoutError is a sentinel error that reports itself as a network timeout,
// without being context.DeadlineExceeded, so that the url.Error.Timeout()
// branch of isTransientErr can be exercised independently.
type netTimeoutError struct{}

func (netTimeoutError) Error() string { return "net: timeout" }
func (netTimeoutError) Timeout() bool { return true }

// TestIsTransientErr exercises every branch of the transport-error classifier.
func TestIsTransientErr(t *testing.T) {
	// Context cancellation is never retryable.
	assert.False(t, isTransientErr(context.Canceled))

	// DeadlineExceeded (e.g. from http.Client.Timeout) is retryable.
	assert.True(t, isTransientErr(context.DeadlineExceeded))

	// url.Error wrapping a net-level timeout is retryable via Timeout().
	urlTimeoutErr := &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: netTimeoutError{},
	}
	assert.True(t, isTransientErr(urlTimeoutErr))

	// url.Error wrapping a plain (non-timeout) error is not retryable.
	urlNonTimeoutErr := &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: errors.New("connection refused"),
	}
	assert.False(t, isTransientErr(urlNonTimeoutErr))
}

// TestBackoffDelay verifies the exponential progression and the cap.
func TestBackoffDelay(t *testing.T) {
	orig := backoffBase
	backoffBase = 100 * time.Millisecond
	t.Cleanup(func() { backoffBase = orig })

	assert.Equal(t, 100*time.Millisecond, backoffDelay(0))
	assert.Equal(t, 200*time.Millisecond, backoffDelay(1))
	assert.Equal(t, 400*time.Millisecond, backoffDelay(2))
	// A large attempt index (100ms * 2^10 = 102.4s) exceeds backoffCap.
	assert.Equal(t, backoffCap, backoffDelay(10))
}

// TestRetry_ContextCancelledDuringBackoff verifies that cancelling the context
// during a retry sleep stops the loop promptly and returns a context error.
func TestRetry_ContextCancelledDuringBackoff(t *testing.T) {
	// Use a longer base so the cancel fires reliably inside the sleep window.
	origBase := backoffBase
	backoffBase = 100 * time.Millisecond
	t.Cleanup(func() { backoffBase = origBase })

	setupRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	// Cancel the context after 20 ms — well inside the 100 ms backoff window.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := New(logging.NewNopLogger())
	_, err := c.GetByISBN(ctx, "9780618640157")
	require.Error(t, err)
	assert.True(
		t,
		strings.Contains(err.Error(), "context") ||
			errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded),
		"expected context error, got: %v", err,
	)
}
