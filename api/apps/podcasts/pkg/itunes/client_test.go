package itunes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
)

func serve(t *testing.T, handler http.HandlerFunc) itunes.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	itunes.SetBaseURL(srv.URL)
	t.Cleanup(func() {
		srv.Close()
		itunes.SetBaseURL("https://itunes.apple.com")
	})
	return itunes.New()
}

func TestSearch_KeepsShowsWithAFeed(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "hardcore history", r.URL.Query().Get("term"))
		assert.Equal(t, "podcast", r.URL.Query().Get("media"))
		_, _ = w.Write([]byte(`{"resultCount":2,"results":[
			{"collectionId":173001861,"collectionName":"Dan Carlin's Hardcore History",
			 "artistName":"Dan Carlin","feedUrl":"https://feeds.example/hh",
			 "artworkUrl600":"https://img.example/hh.jpg",
			 "collectionViewUrl":"https://podcasts.apple.com/podcast/id173001861"},
			{"collectionId":2,"collectionName":"No feed","artistName":"x"}
		]}`))
	})

	got, err := c.Search(context.Background(), "hardcore history")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(173001861), got[0].ID)
	assert.Equal(t, "Dan Carlin's Hardcore History", got[0].Title)
	assert.Equal(t, "Dan Carlin", got[0].Author)
	assert.Equal(t, "https://feeds.example/hh", got[0].FeedURL)
	assert.Equal(t, "https://img.example/hh.jpg", got[0].ArtworkURL)
	assert.Equal(t, "https://podcasts.apple.com/podcast/id173001861", got[0].AppleURL)
}

func TestSearch_UpstreamError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := c.Search(context.Background(), "x")
	assert.Error(t, err)
}

func TestSearch_BadJSON(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	})
	_, err := c.Search(context.Background(), "x")
	assert.Error(t, err)
}

func TestLookup(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lookup", r.URL.Path)
		assert.Equal(t, "42", r.URL.Query().Get("id"))
		_, _ = w.Write([]byte(`{"results":[
			{"collectionId":42,"collectionName":"Show","feedUrl":"https://f.example/42"}
		]}`))
	})

	got, err := c.Lookup(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "Show", got.Title)
	assert.Equal(t, "https://f.example/42", got.FeedURL)
}

func TestLookup_NotFound(t *testing.T) {
	for name, body := range map[string]string{
		"empty":      `{"results":[]}`,
		"no feed":    `{"results":[{"collectionId":42,"collectionName":"Show"}]}`,
		"other show": `{"results":[{"collectionId":7,"feedUrl":"https://f.example/7"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			_, err := c.Lookup(context.Background(), 42)
			assert.ErrorIs(t, err, itunes.ErrNotFound)
		})
	}
}
