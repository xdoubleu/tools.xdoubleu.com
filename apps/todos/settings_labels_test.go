package todos_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
)

func TestAddLabel_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "urgent"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddLabel_InvalidCategory(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddLabelPresetDto{Category: "invalid-cat", Value: "something"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestRemoveLabel_Success(t *testing.T) {
	// First add a label so there's something to remove.
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "to-remove"})
	require.Equal(t, http.StatusSeeOther, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels/label/to-remove/delete",
	)
	tReq2.SetFollowRedirect(false)
	rs := tReq2.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdateLabelColor_Redirect(t *testing.T) {
	// Add label first.
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "color-label"})
	require.Equal(t, http.StatusSeeOther, tReq.Do(t).StatusCode)

	tReq2 := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels/label/color-label/color",
	)
	tReq2.SetContentType(test.FormContentType)
	tReq2.SetFollowRedirect(false)
	tReq2.SetData(dtos.UpdateLabelColorDto{Color: "#ff0000"})
	rs := tReq2.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdateLabelColor_XAsync(t *testing.T) {
	// Add label first.
	addReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	addReq.SetContentType(test.FormContentType)
	addReq.SetFollowRedirect(false)
	addReq.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "async-label"})
	require.Equal(t, http.StatusSeeOther, addReq.Do(t).StatusCode)

	body := strings.NewReader("color=%23abcdef")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/todos/settings/labels/label/async-label/color",
		body,
	)
	req.Header.Set("Content-Type", string(test.FormContentType))
	req.Header.Set("X-Async", "1")
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAddURLPattern_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://github.com/",
		PlatformName: "GitHub",
		Label:        "gh",
		Shortcut:     "gh",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestRemoveURLPattern_Success(t *testing.T) {
	// Add a pattern via the handler.
	addReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns",
	)
	addReq.SetContentType(test.FormContentType)
	addReq.SetFollowRedirect(false)
	addReq.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://linear.app/",
		PlatformName: "Linear",
		Label:        "lin",
		Shortcut:     "lin",
	})
	require.Equal(t, http.StatusSeeOther, addReq.Do(t).StatusCode)

	// Fetch the ID from DB.
	var patternID string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.url_patterns
		WHERE user_id = $1 AND url_prefix = 'https://linear.app/'
		ORDER BY sort_order DESC LIMIT 1`, userID,
	).Scan(&patternID)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns/"+patternID+"/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestRemoveURLPattern_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestUpdateArchive_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/archive",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdateArchiveDto{ArchiveAfterHours: 48})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestAddURLPattern_EmptyPrefix triggers the service validation error in
// AddURLPattern (empty URLPrefix) → covers the `return err` branch in
// addURLPatternHandler and the ErrResourceNotFound/default case in handle.
func TestAddURLPattern_EmptyPrefix(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "",
		PlatformName: "SomePlatform",
		Label:        "sp",
		Shortcut:     "sp",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// TestAddLabel_EmptyValue triggers the service validation error in AddLabelPreset
// when Value is empty → covers the `return err` branch in addLabelHandler.
func TestAddLabel_EmptyValue(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddLabelPresetDto{Category: "label", Value: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}
