//nolint:testpackage // testing unexported service helpers
package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
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
	}

	start := time.Now()
	err := svc.cacheCoverFromURL(t.Context(), uuid.New(), srv.URL)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Less(t, elapsed, 8*time.Second, "must fail fast, not hang")
}
