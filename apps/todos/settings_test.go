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

func createPolicy(t *testing.T, text string, hours int) string {
	t.Helper()
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.policies (owner_user_id, text, reappear_after_hours)
		VALUES ($1, $2, $3)`,
		userID, text, hours,
	)
	require.NoError(t, err)

	var id string
	err = testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.policies
		WHERE owner_user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestUpdatePolicy_Success(t *testing.T) {
	id := createPolicy(t, "Original text", 24)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies/"+id+"/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdatePolicyDto{
		Text:               "Updated text",
		ReappearAfterHours: 48,
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdatePolicy_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies/not-a-uuid/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdatePolicyDto{
		Text:               "Some text",
		ReappearAfterHours: 24,
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestUpdateHideShortcutHints_Enable(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/hide-shortcut-hints",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdateHideShortcutHintsDto{HideShortcutHints: true})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var got bool
	err := testDB.QueryRow(t.Context(),
		`SELECT hide_shortcut_hints FROM todos.user_settings WHERE user_id = $1`,
		userID,
	).Scan(&got)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestUpdateHideShortcutHints_Disable(t *testing.T) {
	_, err := testDB.Exec(t.Context(),
		`INSERT INTO todos.user_settings (user_id, hide_shortcut_hints)
		 VALUES ($1, true)
		 ON CONFLICT (user_id) DO UPDATE SET hide_shortcut_hints = true`,
		userID,
	)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/hide-shortcut-hints",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdateHideShortcutHintsDto{HideShortcutHints: false})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var got bool
	err = testDB.QueryRow(t.Context(),
		`SELECT hide_shortcut_hints FROM todos.user_settings WHERE user_id = $1`,
		userID,
	).Scan(&got)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestUpdatePolicy_EmptyText(t *testing.T) {
	id := createPolicy(t, "Non-empty", 24)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies/"+id+"/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdatePolicyDto{
		Text:               "",
		ReappearAfterHours: 24,
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── Section tests ─────────────────────────────────────────────────────────────

func createSection(t *testing.T, name string) string {
	t.Helper()
	var id string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, $2) RETURNING id::text`,
		userID, name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestAddSection_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/sections",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddSectionDto{Name: "My New Section"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddSection_EmptyName(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/sections",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddSectionDto{Name: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestRemoveSection_Success(t *testing.T) {
	id := createSection(t, "Section To Delete")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/sections/"+id+"/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestRemoveSection_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/sections/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── Policy add/remove tests ───────────────────────────────────────────────────

func TestAddPolicy_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddPolicyDto{
		Text:               "Complete tasks before 5pm",
		ReappearAfterHours: 24,
	})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddPolicy_EmptyText(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddPolicyDto{Text: "", ReappearAfterHours: 24})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestRemovePolicy_Success(t *testing.T) {
	id := createPolicy(t, "Policy To Remove", 48)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies/"+id+"/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestRemovePolicy_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/policies/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── Workspace tests ───────────────────────────────────────────────────────────

func createWorkspace(t *testing.T, name string) string {
	t.Helper()
	var id string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.workspaces (owner_user_id, name)
		VALUES ($1, $2) RETURNING id::text`,
		userID, name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestAddWorkspace_Success(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/workspaces",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddWorkspaceDto{Name: "Work"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddWorkspace_EmptyName(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/workspaces",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.AddWorkspaceDto{Name: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestDeleteWorkspace_Success(t *testing.T) {
	id := createWorkspace(t, "Workspace To Delete")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/workspaces/"+id+"/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestDeleteWorkspace_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/workspaces/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── SetMode tests ─────────────────────────────────────────────────────────────

func TestSetMode_ClearsWorkspace(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/mode",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SetModeDto{WorkspaceID: "", Back: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSetMode_SetsValidWorkspace(t *testing.T) {
	id := createWorkspace(t, "Mode Workspace")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/mode",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SetModeDto{WorkspaceID: id, Back: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSetMode_InvalidWorkspaceUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/mode",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SetModeDto{WorkspaceID: "not-a-uuid", Back: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestSetMode_WithCustomBack(t *testing.T) {
	id := createWorkspace(t, "Back Workspace")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/mode",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SetModeDto{WorkspaceID: id, Back: "/todos"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos", rs.Header.Get("Location"))
}

// ── Label preset tests ────────────────────────────────────────────────────────

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

// ── URL pattern tests ─────────────────────────────────────────────────────────

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

// ── UpdateArchive test ────────────────────────────────────────────────────────

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

// ── addURLPatternHandler service-error coverage ───────────────────────────────

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
