package todos_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
)

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
