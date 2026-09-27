//nolint:testpackage // testing unexported service helpers
package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/logging"
)

// TestCacheCoverFromURL_SlowSourceFailsFast is a regression test for #769: a
// cover source that never responds must not hang the request past
// coverFetchTimeout, since GetBookCover's read-time self-heal calls this
// inline in the public cover handler.
func TestCacheCoverFromURL_SlowSourceFailsFast(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			<-block
		},
	))
	defer func() {
		close(block)
		srv.Close()
	}()

	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		objectStore: objectstore.NewFake(),
		coverClient: newCoverClient("test"),
	}

	start := time.Now()
	err := svc.cacheCoverFromURL(t.Context(), uuid.New(), srv.URL)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Less(t, elapsed, 8*time.Second, "must fail fast, not hang")
}

// pngBytes is a minimal PNG signature, enough for content sniffing.
const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func coverServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			// The upstream header is ignored; the body is sniffed.
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(body)
		},
	))
	t.Cleanup(srv.Close)
	return srv
}

func TestCacheCoverFromURL_StoresSniffedImage(t *testing.T) {
	store := objectstore.NewFake()
	svc := &BookService{ //nolint:exhaustruct // partial
		objectStore: store,
		coverClient: newCoverClient("test"),
	}
	bookID := uuid.New()

	require.NoError(t, svc.cacheCoverFromURL(
		t.Context(), bookID, coverServer(t, []byte(pngBytes)).URL,
	))
	exists, err := store.Exists(t.Context(), bookCoverKey(bookID))
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCacheCoverFromURL_RejectsNonImage(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
		coverClient: newCoverClient("test"),
	}
	srv := coverServer(t, []byte("<html><script>alert(1)</script></html>"))
	assert.Error(t, svc.cacheCoverFromURL(t.Context(), uuid.New(), srv.URL))
}

func TestCacheCoverFromURL_RejectsNonHTTPScheme(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
		coverClient: newCoverClient("test"),
	}
	assert.Error(
		t,
		svc.cacheCoverFromURL(t.Context(), uuid.New(), "file:///etc/passwd"),
	)
}

// The default client, like production, refuses private addresses.
func TestCacheCoverFromURL_BlocksPrivateAddressByDefault(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}
	srv := coverServer(t, []byte(pngBytes))
	err := svc.cacheCoverFromURL(t.Context(), uuid.New(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-public")
}
