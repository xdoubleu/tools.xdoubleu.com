package steam_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/pkg/steam"
)

func TestSearchStore(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/storesearch/", r.URL.Path)
			assert.Equal(t, "outer wilds", r.URL.Query().Get("term"))
			assert.Equal(t, "english", r.URL.Query().Get("l"))
			assert.Equal(t, "BE", r.URL.Query().Get("cc"))
			_, _ = w.Write([]byte(`{"total": 2, "items": [
			{"type": "app", "name": "Outer Wilds", "id": 753640,
			 "tiny_image": "https://img/ow.jpg"},
			{"type": "sub", "name": "Outer Wilds Bundle", "id": 1}
		]}`))
		}),
	)
	t.Cleanup(ts.Close)

	items, err := steam.NewStore(ts.URL).SearchStore(context.Background(), "outer wilds")
	require.NoError(t, err)
	assert.Equal(t, []steam.StoreItem{{
		ID: 753640, Name: "Outer Wilds", Type: "app", ImageURL: "https://img/ow.jpg",
	}}, items)
}

func TestSearchStore_Errors(t *testing.T) {
	for _, body := range []string{"", "{"} {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if body == "" {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				_, _ = w.Write([]byte(body))
			}),
		)
		_, err := steam.NewStore(ts.URL).SearchStore(context.Background(), "x")
		require.Error(t, err)
		ts.Close()
	}
	_, err := steam.NewStore("http://127.0.0.1:1").SearchStore(context.Background(), "x")
	require.Error(t, err)
	_, err = steam.NewStore("::bad").SearchStore(context.Background(), "x")
	require.Error(t, err)
}
