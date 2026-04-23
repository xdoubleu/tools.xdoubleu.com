package icsproxy_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/icsproxy/internal/dtos"
)

// ── feedHandler ──────────────────────────────────────────────────────────────

func TestFeedHandler_ValidToken(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "feed-test-token-001"
	createResp := doRequest(t, http.MethodPost, "/icsproxy/create",
		encodeForm(t, dtos.CreateFilterDto{SourceURL: srv.URL, Token: token}, nil))
	require.Equal(t, http.StatusOK, createResp.StatusCode)

	resp := doRequest(t, http.MethodGet,
		fmt.Sprintf("/icsproxy/%s.ics", token), "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/calendar", resp.Header.Get("Content-Type"))
}

func TestFeedHandler_TokenNotFound(t *testing.T) {
	resp := doRequest(t, http.MethodGet, "/icsproxy/unknown-token-xyz.ics", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestFeedHandler_SourceDown(t *testing.T) {
	srv := calendarServer(t)

	token := "feed-broken-source-001"
	createResp := doRequest(t, http.MethodPost, "/icsproxy/create",
		encodeForm(t, dtos.CreateFilterDto{SourceURL: srv.URL, Token: token}, nil))
	require.Equal(t, http.StatusOK, createResp.StatusCode)

	// Shut down the source server so the feed fetch fails
	srv.Close()

	resp := doRequest(t, http.MethodGet,
		fmt.Sprintf("/icsproxy/%s.ics", token), "")
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}
