package coverfetch_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/coverfetch"
	"tools.xdoubleu.com/internal/config"
)

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		},
	))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch(t *testing.T) {
	srv := serve(t, http.StatusOK, "abc")
	data, err := coverfetch.New("test").Fetch(t.Context(), srv.URL, 10)
	require.NoError(t, err)
	assert.Equal(t, "abc", string(data))
}

func TestFetch_TooLarge(t *testing.T) {
	srv := serve(t, http.StatusOK, "abcdef")
	_, err := coverfetch.New("test").Fetch(t.Context(), srv.URL, 3)
	require.ErrorContains(t, err, "exceeds 3 bytes")
}

func TestFetch_BadStatus(t *testing.T) {
	srv := serve(t, http.StatusNotFound, "")
	_, err := coverfetch.New("test").Fetch(t.Context(), srv.URL, 10)
	require.ErrorContains(t, err, "returned 404")
}

func TestFetch_BadURL(t *testing.T) {
	_, err := coverfetch.New("test").Fetch(t.Context(), "http://bad\x7f", 10)
	require.ErrorContains(t, err, "build cover request")
}

func TestFetch_BlocksPrivateInProduction(t *testing.T) {
	srv := serve(t, http.StatusOK, "abc")
	_, err := coverfetch.New(config.ProdEnv).Fetch(t.Context(), srv.URL, 10)
	require.ErrorContains(t, err, "non-public")
}
