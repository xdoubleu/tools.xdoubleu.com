package runner_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARequestBodyOverTheCapIsRefusedAndNeverFullyForwarded(t *testing.T) {
	var forwarded int64
	p, up := proxyFor(t, nil)
	up.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})
	front := httptest.NewServer(p)
	defer front.Close()

	const over = 65 << 20
	resp, err := http.Post(front.URL+"/anthropic/v1/messages", "application/octet-stream",
		strings.NewReader(strings.Repeat("a", over)))
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Less(t, forwarded, int64(over))
}
