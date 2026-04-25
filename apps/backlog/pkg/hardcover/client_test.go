//nolint:testpackage //needs internal access to override baseURL for testing
package hardcover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
)

func setupTestServer(t *testing.T, payload any) {
	t.Helper()
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(payload))
		}),
	)
	t.Cleanup(func() {
		srv.Close()
		baseURL = "https://api.hardcover.app/v1/graphql"
	})
	baseURL = srv.URL
}

func TestSearch_ReturnsMappedBooks(t *testing.T) {
	isbn13 := "9780593099322"
	coverURL := "https://example.com/cover.jpg"
	setupTestServer(t, map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"hits": []map[string]any{
						{
							"document": map[string]any{
								"id":    "42",
								"title": "Project Hail Mary",
								"contributions": []map[string]any{
									{"author": map[string]any{"name": "Andy Weir"}},
								},
								"description": nil,
								"default_physical_edition": map[string]any{
									"isbn_13": isbn13,
									"isbn_10": nil,
									"image":   map[string]any{"url": coverURL},
								},
							},
						},
					},
				},
			},
		},
	})

	c := New(logging.NewNopLogger(), "test-token")
	results, err := c.Search(context.Background(), "hail mary")
	require.NoError(t, err)
	require.Len(t, results, 1)

	book := results[0]
	assert.Equal(t, "hardcover", book.Provider)
	assert.Equal(t, "42", book.ProviderID)
	assert.Equal(t, "Project Hail Mary", book.Title)
	assert.Equal(t, []string{"Andy Weir"}, book.Authors)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, isbn13, *book.ISBN13)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, coverURL, *book.CoverURL)
}

func TestSearch_GraphQLError(t *testing.T) {
	setupTestServer(t, map[string]any{
		"errors": []map[string]any{
			{"message": "unauthorized"},
		},
	})

	c := New(logging.NewNopLogger(), "bad-token")
	_, err := c.Search(context.Background(), "anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSearch_EmptyResults(t *testing.T) {
	setupTestServer(t, map[string]any{
		"data": map[string]any{
			"search": map[string]any{
				"results": map[string]any{
					"hits": []any{},
				},
			},
		},
	})

	c := New(logging.NewNopLogger(), "test-token")
	results, err := c.Search(context.Background(), "xyz")
	require.NoError(t, err)
	assert.Empty(t, results)
}
