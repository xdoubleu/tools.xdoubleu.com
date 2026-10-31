//nolint:testpackage // testing unexported redactKoboToken
package books

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKoboAllowedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer x")
	assert.Nil(t, koboAllowedHeaders(h, koboLogResponseHeaders),
		"no allowlisted header present must yield nil")

	h.Set("X-Kobo-Sync", "continue")
	assert.Equal(t, map[string]string{"x-kobo-sync": "continue"},
		koboAllowedHeaders(h, koboLogResponseHeaders))
}

// TestKoboResponseRecorder_SnapshotsHeadersWhenSent: headers set after the
// response is written never reach the device, so they must not be logged.
func TestKoboResponseRecorder_SnapshotsHeadersWhenSent(t *testing.T) {
	//nolint:exhaustruct // zero values are the intended initial state
	holder := &koboLogHolder{enabled: true}
	rec := &koboResponseRecorder{ResponseWriter: httptest.NewRecorder(), holder: holder}

	rec.Header().Set("x-kobo-sync", "continue")
	_, _ = rec.Write([]byte("[]"))
	rec.Header().Set("x-kobo-synctoken", "too-late")
	rec.WriteHeader(http.StatusTeapot)

	assert.Equal(t, map[string]string{"x-kobo-sync": "continue"}, holder.respHeaders)
}

func TestRedactKoboToken(t *testing.T) {
	assert.Equal(t,
		"/books/kobo/redacted/v1/library/sync",
		redactKoboToken("/books/kobo/AbC123secret/v1/library/sync", "AbC123secret"),
	)
	assert.Equal(t,
		"/books/kobo//v1/x",
		redactKoboToken("/books/kobo//v1/x", ""),
		"empty token must leave the path unchanged",
	)
}
