package todos_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
)

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
