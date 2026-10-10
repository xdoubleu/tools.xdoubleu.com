//nolint:testpackage // testing unexported redactKoboToken/koboUpstreamClient
package books

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// TestKoboUpstreamClient_TimesOutOnSlowUpstream is the regression test for the
// "device sync hangs" bug: koboUpstreamClient must give up on a stalled
// upstream instead of blocking forever. Shrinks the shared client's timeout
// for the duration of the test so it doesn't need to wait out the real
// production value.
func TestKoboUpstreamClient_TimesOutOnSlowUpstream(t *testing.T) {
	original := koboUpstreamClient.Timeout
	koboUpstreamClient.Timeout = 50 * time.Millisecond
	t.Cleanup(func() { koboUpstreamClient.Timeout = original })

	upstream := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done() // never respond until the client gives up
		},
	))
	t.Cleanup(upstream.Close)

	req, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	assert.NoError(t, err)

	start := time.Now()
	_, doErr := koboUpstreamClient.Do(req)
	elapsed := time.Since(start)

	assert.Error(t, doErr, "a stalled upstream must fail, not hang forever")
	assert.Less(t, elapsed, 2*time.Second,
		"koboUpstreamClient.Timeout must bound the call to a stalled upstream")
}
