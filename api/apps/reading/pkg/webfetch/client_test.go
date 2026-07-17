package webfetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

func newClient() webfetch.Client {
	return webfetch.New(logging.NewNopLogger())
}

// opts builds fully-initialized Options with just a size cap.
func opts(maxBytes int64) webfetch.Options {
	return webfetch.Options{
		ETag:         "",
		LastModified: "",
		MaxBytes:     maxBytes,
		Accept:       "",
	}
}

func TestGet_Basic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.NotEmpty(t, r.Header.Get("User-Agent"))
			w.Header().Set("Content-Type", "text/HTML; charset=utf-8")
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 00:00:00 GMT")
			_, _ = w.Write([]byte("<html>hi</html>"))
		},
	))
	t.Cleanup(ts.Close)

	res, err := newClient().Get(context.Background(), ts.URL, opts(0))
	require.NoError(t, err)
	assert.Equal(t, "<html>hi</html>", string(res.Body))
	assert.Equal(t, "text/html", res.ContentType)
	assert.Equal(t, `"v1"`, res.ETag)
	assert.Equal(t, "Mon, 01 Jan 2024 00:00:00 GMT", res.LastModified)
	assert.False(t, res.NotModified)
}

func TestGet_ConditionalNotModified(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("If-None-Match") == `"v1"` {
				assert.Equal(
					t, "Mon, 01 Jan 2024 00:00:00 GMT",
					r.Header.Get("If-Modified-Since"),
				)
				w.WriteHeader(http.StatusNotModified)
				return
			}
			_, _ = w.Write([]byte("fresh"))
		},
	))
	t.Cleanup(ts.Close)

	conditional := opts(0)
	conditional.ETag = `"v1"`
	conditional.LastModified = "Mon, 01 Jan 2024 00:00:00 GMT"
	res, err := newClient().Get(context.Background(), ts.URL, conditional)
	require.NoError(t, err)
	assert.True(t, res.NotModified)
	assert.Empty(t, res.Body)
}

func TestGet_TooLarge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		},
	))
	t.Cleanup(ts.Close)

	_, err := newClient().Get(context.Background(), ts.URL, opts(99))
	assert.ErrorIs(t, err, webfetch.ErrTooLarge)
}

func TestGet_ExactlyAtCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		},
	))
	t.Cleanup(ts.Close)

	res, err := newClient().Get(context.Background(), ts.URL, opts(100))
	require.NoError(t, err)
	assert.Len(t, res.Body, 100)
}

func TestGet_Redirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("landed"))
		},
	))
	t.Cleanup(target.Close)
	src := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/final", http.StatusFound)
		},
	))
	t.Cleanup(src.Close)

	res, err := newClient().Get(context.Background(), src.URL, opts(0))
	require.NoError(t, err)
	assert.Equal(t, "landed", string(res.Body))
	assert.Equal(t, target.URL+"/final", res.FinalURL)
}

func TestGet_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		},
	))
	t.Cleanup(ts.Close)

	_, err := newClient().Get(context.Background(), ts.URL, opts(0))
	assert.ErrorIs(t, err, webfetch.ErrStatus)
}

func TestGet_SchemeRejected(t *testing.T) {
	for _, u := range []string{"ftp://example.com/x", "file:///etc/passwd"} {
		_, err := newClient().Get(context.Background(), u, opts(0))
		assert.ErrorIs(t, err, webfetch.ErrScheme, u)
	}
}
