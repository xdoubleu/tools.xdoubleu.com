package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gen/books/v1/booksv1connect"
	"tools.xdoubleu.com/internal/communication/httptools"
)

// kamalDefaultResponseTimeout is Kamal's proxy.response_timeout when unset.
const kamalDefaultResponseTimeout = 30 * time.Second

func proxyResponseTimeout(t *testing.T) time.Duration {
	t.Helper()
	raw, err := os.ReadFile("../../../config/deploy.api.yml")
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^\s+response_timeout:\s*(\d+)\s*$`).FindSubmatch(raw)
	if m == nil {
		return kamalDefaultResponseTimeout
	}
	secs, err := strconv.Atoi(string(m[1]))
	require.NoError(t, err)
	return time.Duration(secs) * time.Second
}

// Each limit must fire before the one outside it: the outer one cuts the
// connection with no log.
func TestRequestBudgetFitsServerAndProxyTimeouts(t *testing.T) {
	proxy := proxyResponseTimeout(t)

	assert.Less(t, httptools.UpstreamTimeout, httptools.RequestBudget)
	assert.LessOrEqual(t,
		httptools.RequestBudget+httptools.WriteSlack, httpWriteTimeout)
	assert.Less(t, httpWriteTimeout, proxy)
	assert.Less(t, httptools.LongRequestBudget+httptools.WriteSlack, proxy)
}

func post(t *testing.T, srv *httptest.Server, path string) (string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, srv.URL+path, strings.NewReader("{}"),
	)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

// Through the real middleware chain, a long route's response outlives
// WriteTimeout and any other route's is still cut. A wrapper without Unwrap
// would leave the long route cut too.
func TestMiddlewareLongRouteOutlivesWriteTimeout(t *testing.T) {
	const writeTimeout = 100 * time.Millisecond
	late := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * writeTimeout)
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewUnstartedServer(testApp.middleware().Then(late))
	srv.Config.WriteTimeout = writeTimeout
	srv.Start()
	t.Cleanup(srv.Close)

	body, err := post(t, srv, booksv1connect.BookFilesServiceFinalizeBookUploadProcedure)
	require.NoError(t, err)
	assert.Equal(t, "ok", body)

	_, err = post(t, srv, booksv1connect.BookFilesServiceGetBookFileProcedure)
	require.Error(t, err)
}
