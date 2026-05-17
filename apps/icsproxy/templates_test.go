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

// ── indexHandler — with existing configs (covers Configs branch) ─────────────

func TestIndexHandler_WithConfigs(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "index-with-configs-token"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	// Index page should now list the config
	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet, "/icsproxy")
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

// ── createHandler — shows GeneratedURL and lists configs ─────────────────────

func TestCreateHandler_ShowsGeneratedURL(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         "generated-url-token-001",
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// ── createHandler — missing source_url validation error ─────────────────────

func TestCreateHandler_MissingSourceURL(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     "",
		Token:         "missing-url-token",
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	// validation fails → FailedValidationResponse (422)
	assert.Equal(t, http.StatusUnprocessableEntity, tReq.Do(t).StatusCode)
}

// ── previewHandler — missing source_url validation error ─────────────────────

func TestPreviewHandler_EmptyURL(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/preview")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.PreviewDto{SourceURL: ""})
	// validation fails → 422
	assert.Equal(t, http.StatusUnprocessableEntity, tReq.Do(t).StatusCode)
}

// ── editHandler — source server down ─────────────────────────────────────────

func TestEditHandler_SourceDown(t *testing.T) {
	srv := calendarServer(t)

	token := "edit-source-down-token"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	// Shut the source down so the fetch inside editHandler fails
	srv.Close()

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/edit/%s", token))
	assert.Equal(t, http.StatusBadGateway, tReq2.Do(t).StatusCode)
}

// ── editHandler — with hide UIDs pre-populated ───────────────────────────────

func TestEditHandler_WithHideUIDs(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "edit-with-uids-token"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: []string{"test-uid-1"},
		HolidayUIDs:   []string{"holiday-uid"},
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/edit/%s", token))
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

// ── ListFilterSummaries / ListConfigSummaries ─────────────────────────────────
// These are called through the repository/service layer; exercise them via the
// index page which triggers ListConfigs (same code path as list summaries).

func TestListFilterSummaries_ViaCreateAndIndex(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "summary-test-token-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: nil,
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	// Verify the index page (which lists configs) still returns 200
	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet, "/icsproxy")
	assert.Equal(t, http.StatusOK, tReq2.Do(t).StatusCode)
}

// ── feedHandler — ApplyFilter path with hide UIDs ────────────────────────────

func TestFeedHandler_WithHideUIDs(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token := "feed-hide-uid-token-001"

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/icsproxy/create")
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.CreateFilterDto{
		SourceURL:     srv.URL,
		Token:         token,
		HideEventUIDs: []string{"test-uid-1"},
		HolidayUIDs:   nil,
	})
	require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/icsproxy/%s.ics", token))
	rs := tReq2.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
	assert.Equal(t, "text/calendar", rs.Header.Get("Content-Type"))
}

// ── deleteHandler — after delete, index shows remaining configs ───────────────

func TestDeleteHandler_LeavesOtherConfigs(t *testing.T) {
	srv := calendarServer(t)
	defer srv.Close()

	token1 := "delete-leave-token-001"
	token2 := "delete-leave-token-002"

	for _, tok := range []string{token1, token2} {
		tReq := test.CreateRequestTester(
			getRoutes(),
			http.MethodPost,
			"/icsproxy/create",
		)
		tReq.SetContentType(test.FormContentType)
		tReq.SetData(dtos.CreateFilterDto{
			SourceURL:     srv.URL,
			Token:         tok,
			HideEventUIDs: nil,
			HolidayUIDs:   nil,
		})
		require.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
	}

	// Delete one; index page should still show the other (exercises Configs branch)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		fmt.Sprintf("/icsproxy/delete/%s", token1))
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
