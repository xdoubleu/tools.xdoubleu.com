package googlebooks_test

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

	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
)

func TestMain(m *testing.M) {
	// Speed up retries in all tests in this package.
	googlebooks.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

// buildServer starts an httptest.Server that serves handler and overrides the
// package-level baseURL to point at it. The returned func closes the server and
// restores the original baseURL.
func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	googlebooks.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		googlebooks.SetBaseURL("https://www.googleapis.com/books/v1")
	}
}

func TestGetByISBN_Found(t *testing.T) {
	body := volumesResponse(t, []volumeInfoFixture{{
		Title:       "2001: A Space Odyssey",
		Authors:     []string{"Arthur C. Clarke"},
		Description: "A novel.",
		PageCount:   221,
		Thumbnail:   "https://books.google.com/cover.jpg",
		ISBN13:      "9780451457998",
		ISBN10:      "0451457994",
	}})

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	got, err := c.GetByISBN(context.Background(), "9780451457998")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "2001: A Space Odyssey", got.Title)
	assert.Equal(t, []string{"Arthur C. Clarke"}, got.Authors)
	require.NotNil(t, got.ISBN13)
	assert.Equal(t, "9780451457998", *got.ISBN13)
	require.NotNil(t, got.Description)
	assert.Equal(t, "A novel.", *got.Description)
	require.NotNil(t, got.PageCount)
	assert.Equal(t, 221, *got.PageCount)
	require.NotNil(t, got.CoverURL)
	assert.Equal(t, "https://books.google.com/cover.jpg", *got.CoverURL)
}

func TestGetByISBN_ForcesHTTPS(t *testing.T) {
	body := volumesResponse(t, []volumeInfoFixture{
		{ //nolint:exhaustruct // only fields relevant to this test
			Title:     "Test",
			Authors:   []string{"Author"},
			Thumbnail: "http://books.google.com/cover.jpg",
		},
	})

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	got, err := c.GetByISBN(context.Background(), "9780000000000")
	require.NoError(t, err)
	require.NotNil(t, got.CoverURL)
	assert.True(t,
		len(*got.CoverURL) > 8 && (*got.CoverURL)[:8] == "https://",
		"cover URL must use HTTPS, got %s", *got.CoverURL,
	)
}

func TestGetByISBN_NotFound(t *testing.T) {
	body := `{"totalItems":0,"items":null}`

	cleanup := buildServer(jsonHandler(json.RawMessage(body)))
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	got, err := c.GetByISBN(context.Background(), "9780000000000")
	require.ErrorIs(t, err, googlebooks.ErrNotFound)
	assert.Nil(t, got)
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
			body := volumesResponse(t, []volumeInfoFixture{
				{ //nolint:exhaustruct // only fields relevant to this test
					Title:   "Retry Book",
					Authors: []string{"Author"},
					ISBN13:  "9780000000001",
				},
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(mustJSON(t, body))
		}),
	)
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	got, err := c.GetByISBN(context.Background(), "9780000000001")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Retry Book", got.Title)
	assert.GreaterOrEqual(t, attempts, 3, "expected at least 3 attempts (2 retries)")
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
			body := volumesResponse(t, []volumeInfoFixture{
				{ //nolint:exhaustruct // only fields relevant to this test
					Title:   "Rate Book",
					Authors: []string{"Author"},
					ISBN13:  "9780000000002",
				},
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(mustJSON(t, body))
		}),
	)
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	got, err := c.GetByISBN(context.Background(), "9780000000002")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.GreaterOrEqual(t, attempts, 2)
}

func TestSearch_ReturnsResults(t *testing.T) {
	body := volumesResponse(t, []volumeInfoFixture{
		{ //nolint:exhaustruct // only fields relevant to this test
			Title:   "Space Odyssey",
			Authors: []string{"Clarke"},
			ISBN13:  "9780000000010",
		},
		{ //nolint:exhaustruct // only fields relevant to this test
			Title:   "Another Book",
			Authors: []string{"Smith"},
		},
	})

	cleanup := buildServer(jsonHandler(body))
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	results, err := c.Search(
		context.Background(),
		"intitle:Space+Odyssey inauthor:Clarke",
	)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "Space Odyssey", results[0].Title)
}

func TestSearch_Empty(t *testing.T) {
	body := `{"totalItems":0}`

	cleanup := buildServer(jsonHandler(json.RawMessage(body)))
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "")
	results, err := c.Search(context.Background(), "xyzzyunknownbook")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestGetByISBN_WithAPIKey(t *testing.T) {
	var capturedRawQuery string
	cleanup := buildServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedRawQuery = r.URL.RawQuery
			body := volumesResponse(t, []volumeInfoFixture{
				{ //nolint:exhaustruct // only fields relevant to this test
					Title:   "Keyed Book",
					Authors: []string{"Author"},
					ISBN13:  "9780000099999",
				},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		}),
	)
	defer cleanup()

	c := googlebooks.New(logging.NewNopLogger(), "test-api-key")
	_, err := c.GetByISBN(context.Background(), "9780000099999")
	require.NoError(t, err)
	assert.Contains(t, capturedRawQuery, "key=test-api-key")
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

	c := googlebooks.New(logging.NewNopLogger(), "")
	_, err := c.GetByISBN(context.Background(), "9780000000000")
	require.Error(t, err)
	assert.Equal(t, 1, attempts, "4xx must not trigger retries")
}

func TestGetByISBN_ContextCanceled(t *testing.T) {
	cleanup := buildServer(jsonHandler(json.RawMessage(`{"totalItems":0}`)))
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before making any request

	c := googlebooks.New(logging.NewNopLogger(), "")
	_, err := c.GetByISBN(ctx, "9780000000000")
	require.ErrorIs(t, err, context.Canceled)
}

func TestGetByISBN_NetworkError(t *testing.T) {
	// Point to a port where nothing listens — httpClient.Do will fail with a
	// *url.Error (connection refused), covering the isTransientErr url.Error path.
	googlebooks.SetBaseURL("http://127.0.0.1:1")
	defer googlebooks.SetBaseURL("https://www.googleapis.com/books/v1")

	c := googlebooks.New(logging.NewNopLogger(), "")
	_, err := c.GetByISBN(context.Background(), "9780000000000")
	require.Error(t, err)
}

// --- helpers ---

type volumeInfoFixture struct {
	Title       string
	Authors     []string
	Description string
	PageCount   int
	Thumbnail   string
	ISBN13      string
	ISBN10      string
}

func volumesResponse(t *testing.T, items []volumeInfoFixture) json.RawMessage {
	t.Helper()

	type ii struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	}
	type il struct {
		Thumbnail string `json:"thumbnail"`
	}
	type vi struct {
		Title               string   `json:"title"`
		Authors             []string `json:"authors"`
		Description         string   `json:"description,omitempty"`
		PageCount           int      `json:"pageCount,omitempty"`
		ImageLinks          *il      `json:"imageLinks,omitempty"`
		IndustryIdentifiers []ii     `json:"industryIdentifiers,omitempty"`
	}
	type vol struct {
		VolumeInfo vi `json:"volumeInfo"`
	}
	type resp struct {
		TotalItems int   `json:"totalItems"`
		Items      []vol `json:"items,omitempty"`
	}

	vols := make([]vol, 0, len(items))
	for _, f := range items {
		v := vi{ //nolint:exhaustruct // ImageLinks/IndustryIdentifiers set below
			Title:       f.Title,
			Authors:     f.Authors,
			Description: f.Description,
			PageCount:   f.PageCount,
		}
		if f.Thumbnail != "" {
			v.ImageLinks = &il{Thumbnail: f.Thumbnail}
		}
		if f.ISBN13 != "" {
			v.IndustryIdentifiers = append(
				v.IndustryIdentifiers,
				ii{Type: "ISBN_13", Identifier: f.ISBN13},
			)
		}
		if f.ISBN10 != "" {
			v.IndustryIdentifiers = append(
				v.IndustryIdentifiers,
				ii{Type: "ISBN_10", Identifier: f.ISBN10},
			)
		}
		vols = append(vols, vol{VolumeInfo: v})
	}

	return mustJSON(t, resp{TotalItems: len(vols), Items: vols})
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
