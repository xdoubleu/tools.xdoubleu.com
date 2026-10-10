package hardcover_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/internal/logging"
)

// routeByQuery answers each request with the body whose key occurs in the
// GraphQL query text; unmatched queries get a GraphQL error.
func routeByQuery(t *testing.T, bodies map[string]any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))

		w.Header().Set("Content-Type", "application/json")
		for key, body := range bodies {
			if strings.Contains(req.Query, key) {
				_, _ = w.Write(mustJSON(t, body))
				return
			}
		}
		_, _ = w.Write(mustJSON(t, map[string]any{
			"errors": []map[string]any{{"message": "unexpected query"}},
		}))
	})
}

func seriesLink(name string, position any, primary any) map[string]any {
	return map[string]any{
		"position": position,
		"series":   map[string]any{"name": name, "primary_books_count": primary},
	}
}

func TestSearch_IncludesSeries(t *testing.T) {
	b := bookJSON(bookFixture{ //nolint:exhaustruct // only fields under test
		ID: 1, Title: "Mort", Authors: []string{"Terry Pratchett"},
	})
	b["book_series"] = []map[string]any{
		seriesLink("Discworld: Death", 1, 5),
		seriesLink("Discworld", 4, 41),
		seriesLink("Unordered", nil, 99),
	}

	cleanup := buildServer(routeByQuery(t, map[string]any{
		"SearchBookIDs": map[string]any{
			"data": map[string]any{"search": map[string]any{"ids": []int{1}}},
		},
		"BooksByIDs": map[string]any{
			"data": map[string]any{"books": []map[string]any{b}},
		},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.Search(context.Background(), `intitle:"Mort"`)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].Series)
	assert.Equal(t, "Discworld", got[0].Series.Name)
	require.NotNil(t, got[0].Series.Position)
	assert.InDelta(t, 4.0, *got[0].Series.Position, 0)
	require.NotNil(t, got[0].Series.Total)
	assert.Equal(t, 41, *got[0].Series.Total)
}

func TestSearch_NoSeries(t *testing.T) {
	cleanup := buildServer(searchIDsThenBooksHandler(t, []int{1}, searchResponse(
		t, []bookFixture{{ //nolint:exhaustruct // only fields under test
			ID: 1, Title: "Standalone",
		}},
	)))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.Search(context.Background(), `intitle:"Standalone"`)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Nil(t, got[0].Series)
}

func isbnWithBookID(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{"data": map[string]any{"editions": []map[string]any{{
		"title":   "Mort",
		"isbn_13": "9780552131063",
		"book":    map[string]any{"id": 7, "title": "Mort"},
	}}}}
}

func TestGetByISBN_LooksUpSeries(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"BookByISBN": isbnWithBookID(t),
		"BookSeries": map[string]any{"data": map[string]any{
			"book_series": []map[string]any{seriesLink("Discworld", 4, 0)},
		}},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780552131063")
	require.NoError(t, err)
	require.NotNil(t, got.Series)
	assert.Equal(t, "Discworld", got.Series.Name)
	assert.Nil(t, got.Series.Total, "a zero count reads as unknown")
}

func TestGetByISBN_SeriesGraphQLErrorIgnoresData(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"BookByISBN": isbnWithBookID(t),
		"BookSeries": map[string]any{
			"errors": []map[string]any{{"message": "partial"}},
			"data": map[string]any{
				"book_series": []map[string]any{seriesLink("Discworld", 4, 41)},
			},
		},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780552131063")
	require.NoError(t, err)
	assert.Nil(t, got.Series)
}

func TestSearch_EqualSeriesKeepsFirst(t *testing.T) {
	b := bookJSON(bookFixture{ //nolint:exhaustruct // only fields under test
		ID: 1, Title: "Mort",
	})
	b["book_series"] = []map[string]any{
		seriesLink("First", 1, 5),
		seriesLink("Second", 2, 5),
	}
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"SearchBookIDs": map[string]any{
			"data": map[string]any{"search": map[string]any{"ids": []int{1}}},
		},
		"BooksByIDs": map[string]any{
			"data": map[string]any{"books": []map[string]any{b}},
		},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.Search(context.Background(), `intitle:"Mort"`)
	require.NoError(t, err)
	assert.Equal(t, "First", got[0].Series.Name)
}

func TestGetByISBN_SeriesLookupFailureIsNonFatal(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"BookByISBN": isbnWithBookID(t),
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetByISBN(context.Background(), "9780552131063")
	require.NoError(t, err)
	assert.Equal(t, "Mort", got.Title)
	assert.Nil(t, got.Series)
}

func TestGetSeries_Found(t *testing.T) {
	vol := func(pos any, title string) map[string]any {
		return map[string]any{
			"position": pos,
			"book": map[string]any{
				"id":                  1,
				"title":               title,
				"pages":               250,
				"cached_contributors": contributorsJSON([]string{"Terry Pratchett"}, false),
				"cached_image":        map[string]any{"url": "https://hardcover.app/c.jpg"},
				"editions":            []map[string]any{{"isbn_13": "9780552124751"}},
			},
		}
	}
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"SeriesByName": map[string]any{"data": map[string]any{
			"series": []map[string]any{{
				"id": 3, "name": "Discworld", "primary_books_count": 41,
			}},
		}},
		"SeriesBooks": map[string]any{"data": map[string]any{
			"book_series": []map[string]any{
				vol(1, "The Colour of Magic"),
				vol(nil, "skipped"),
				vol(2.5, "A Novella"),
			},
		}},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	got, err := c.GetSeries(context.Background(), "Discworld")
	require.NoError(t, err)
	assert.Equal(t, "Discworld", got.Name)
	require.NotNil(t, got.Total)
	assert.Equal(t, 41, *got.Total)
	require.Len(t, got.Books, 2)
	assert.Equal(t, "The Colour of Magic", got.Books[0].Book.Title)
	assert.Equal(t, []string{"Terry Pratchett"}, got.Books[0].Book.Authors)
	require.NotNil(t, got.Books[0].Book.ISBN13)
	assert.Equal(t, "9780552124751", *got.Books[0].Book.ISBN13)
	assert.InDelta(t, 2.5, got.Books[1].Position, 0)
}

func TestGetSeries_NotFound(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"SeriesByName": map[string]any{"data": map[string]any{
			"series": []map[string]any{},
		}},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetSeries(context.Background(), "Nope")
	assert.ErrorIs(t, err, hardcover.ErrNotFound)
}

func TestGetSeries_BooksQueryFails(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{
		"SeriesByName": map[string]any{"data": map[string]any{
			"series": []map[string]any{{"id": 3, "name": "Discworld"}},
		}},
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetSeries(context.Background(), "Discworld")
	require.Error(t, err)
}

func TestGetSeries_GraphQLError(t *testing.T) {
	cleanup := buildServer(routeByQuery(t, map[string]any{}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetSeries(context.Background(), "Discworld")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected query")
}

func TestGetSeries_HTTPError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer cleanup()

	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetSeries(context.Background(), "Discworld")
	require.Error(t, err)
}
