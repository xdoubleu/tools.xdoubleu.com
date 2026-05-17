package todos_test

import (
	"net/http"
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
