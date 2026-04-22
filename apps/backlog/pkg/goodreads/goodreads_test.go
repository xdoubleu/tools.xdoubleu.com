package goodreads_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return f(r)
}

func mockGoodreadsServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return orig.RoundTrip(req2)
		},
	)
	t.Cleanup(func() {
		http.DefaultTransport = orig
		srv.Close()
	})
}

// profileHTML ends with /<id>.jpg so GetUserID extracts "98765432".
const profileHTML = "<html><body>" +
	`<img class="profilePictureIcon" src="http://img.example.com/98765432.jpg" />` +
	"</body></html>"

func TestGetUserID(t *testing.T) {
	mockGoodreadsServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(profileHTML))
	})

	client := goodreads.New(logging.NewNopLogger())
	userID, err := client.GetUserID("https://www.goodreads.com/user/show/98765432")
	require.NoError(t, err)
	assert.Equal(t, "98765432", *userID)
}

const shelfListHTML = `<html><body>
<div id="paginatedShelfList">
  <li><a href="?shelf=read">read</a></li>
  <li><a href="?shelf=want-to-read">want to read</a></li>
  <li class="horizontalGreyDivider"></li>
  <li><a href="?shelf=fiction">fiction</a></li>
</div>
</body></html>`

const emptyBooksHTML = `<html><body></body></html>`

func TestGetBooks(t *testing.T) {
	mockGoodreadsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/review/list/98765432" && r.URL.RawQuery == "" {
			_, _ = w.Write([]byte(shelfListHTML))
			return
		}
		_, _ = w.Write([]byte(emptyBooksHTML))
	})

	client := goodreads.New(logging.NewNopLogger())
	books, err := client.GetBooks(context.Background(), "98765432")
	require.NoError(t, err)
	assert.NotNil(t, books)
}
