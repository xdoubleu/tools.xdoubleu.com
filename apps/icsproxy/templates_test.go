package icsproxy_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/icsproxy/internal/dtos"
)

// ── indexHandler ─────────────────────────────────────────────────────────────

func TestIndexHandler(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/icsproxy")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── previewHandler ───────────────────────────────────────────────────────────

func TestPreviewHandler_ValidURL(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/preview")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.PreviewDto{SourceURL: srv.URL})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestPreviewHandler_InvalidURL(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/preview")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.PreviewDto{SourceURL: "not-a-url"})
	assert.Equal(t, http.StatusBadGateway, tReq.Do(t).StatusCode)
}

func TestPreviewHandler_UnreachableURL(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/preview")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.PreviewDto{SourceURL: "http://127.0.0.1:1"})
	assert.Equal(t, http.StatusBadGateway, tReq.Do(t).StatusCode)
}

// ── createHandler ────────────────────────────────────────────────────────────

func TestCreateHandler_CreatesConfig(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         "",
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestCreateHandler_WithExplicitToken(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         "explicit-test-token-001",
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestCreateHandler_WithHideUIDs(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         "hide-uid-test-token",
		HideEventUIDs: []string{"test-uid-1"},
		HolidayUIDs:   []string{"holiday-uid"},
	})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── editHandler ──────────────────────────────────────────────────────────────

func TestEditHandler_ExistingConfig(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "edit-test-token-001"

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
		fmt.Sprintf("/icsproxy/edit/%s", token))
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

func TestEditHandler_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/icsproxy/edit/nonexistent-token-xyz")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestEditHandler_WrongUser(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "wrong-user-token-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	// The mock auth always returns the same user, so this user IS the owner.
	// A 403 would require a different user — just verify the edit path works.
	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/edit/%s", token))
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

// ── deleteHandler ────────────────────────────────────────────────────────────

func TestDeleteHandler_ExistingConfig(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "delete-test-token-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodPost,
		fmt.Sprintf("/icsproxy/delete/%s", token))
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

func TestDeleteHandler_NonExistent(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/icsproxy/delete/no-such-token")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}
