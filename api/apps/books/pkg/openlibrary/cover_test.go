//nolint:testpackage //needs internal access to override baseURL for testing
package openlibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

func TestFetchCover_Success(t *testing.T) {
	imgBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG magic bytes

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.RawQuery, "default=false")
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(imgBytes)
		}),
	)
	t.Cleanup(srv.Close)

	c := New(logging.NewNopLogger())
	data, ct, err := c.FetchCover(context.Background(), srv.URL+"/b/id/123-L.jpg")
	require.NoError(t, err)
	assert.Equal(t, imgBytes, data)
	assert.Equal(t, "image/jpeg", ct)
}

func TestFetchCover_NotFound(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	t.Cleanup(srv.Close)

	c := New(logging.NewNopLogger())
	_, _, err := c.FetchCover(context.Background(), srv.URL+"/b/id/999-L.jpg")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCoverNotFound)
}

func TestFetchCover_ServerError(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	t.Cleanup(srv.Close)

	c := New(logging.NewNopLogger())
	_, _, err := c.FetchCover(context.Background(), srv.URL+"/b/id/1-L.jpg")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCoverNotFound)
}

func TestFetchCover_DefaultFalseAppended(t *testing.T) {
	var got string
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.RawQuery
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("img"))
		}),
	)
	t.Cleanup(srv.Close)

	c := New(logging.NewNopLogger())
	// URL with existing query param — default=false should be appended, not replace.
	_, _, err := c.FetchCover(context.Background(), srv.URL+"/b/id/1-L.jpg?foo=bar")
	require.NoError(t, err)
	assert.Contains(t, got, "default=false")
	assert.Contains(t, got, "foo=bar")
}
