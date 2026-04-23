package icsproxy_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/apps/icsproxy/internal/dtos"
)

// ── feedHandler ──────────────────────────────────────────────────────────────

func TestFeedHandler_ValidToken(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "feed-test-token-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/%s.ics", token))
	rs := tReq2.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
	assert.Equal(t, "text/calendar", rs.Header.Get("Content-Type"))
}

func TestFeedHandler_TokenNotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/icsproxy/unknown-token-xyz.ics")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestFeedHandler_SourceDown(t *testing.T) {
	srv := calendarServer(t)

	token := "feed-broken-source-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	// Shut down the source server so the feed fetch fails
	srv.Close()

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/%s.ics", token))
	assert.Equal(t, http.StatusBadGateway, tReq2.Do(t).StatusCode)
}
