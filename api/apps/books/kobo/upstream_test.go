//nolint:testpackage // mutates the unexported upstreamClient
package kobo

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestUpstreamClient_TimesOutOnSlowUpstream is the regression test for the
// "device sync hangs" bug: upstreamClient must give up on a stalled
// upstream instead of blocking forever. Shrinks the shared client's timeout
// for the duration of the test so it doesn't need to wait out the real
// production value.
func TestUpstreamClient_TimesOutOnSlowUpstream(t *testing.T) {
	original := upstreamClient.Timeout
	upstreamClient.Timeout = 50 * time.Millisecond
	t.Cleanup(func() { upstreamClient.Timeout = original })

	upstream := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done() // never respond until the client gives up
		},
	))
	t.Cleanup(upstream.Close)

	req, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	assert.NoError(t, err)

	start := time.Now()
	_, doErr := upstreamClient.Do(req)
	elapsed := time.Since(start)

	assert.Error(t, doErr, "a stalled upstream must fail, not hang forever")
	assert.Less(t, elapsed, 2*time.Second,
		"upstreamClient.Timeout must bound the call to a stalled upstream")
}
