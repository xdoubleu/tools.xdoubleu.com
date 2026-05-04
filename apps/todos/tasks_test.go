package todos_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
)

func TestListTasks_Empty(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestListDone_Empty(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/done")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestListArchive_Empty(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/archive")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestNewTaskForm(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/new")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestCreateAndViewTask(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct // only Title needed for this test
	tReq.SetData(dtos.SaveTaskDto{Title: "Test task"})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	id := createTask(t, "Directly inserted task")
	tView := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/"+id)
	rs = tView.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestCompleteAndReopenTask(t *testing.T) {
	id := createTask(t, "Task to complete")

	complete := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	complete.SetFollowRedirect(false)
	rs := complete.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var status string
	err := testDB.QueryRow(t.Context(),
		`SELECT status FROM todos.tasks WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	reopen := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/reopen",
	)
	reopen.SetFollowRedirect(false)
	rs = reopen.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	err = testDB.QueryRow(t.Context(),
		`SELECT status FROM todos.tasks WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "open", status)
}

func TestDeleteTask(t *testing.T) {
	id := createTask(t, "Task to delete")

	del := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/delete",
	)
	del.SetFollowRedirect(false)
	rs := del.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var count int
	err := testDB.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM todos.tasks WHERE id = $1`, id,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestQuickAdd_PlainTitle(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{Input: "Buy milk"})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var title string
	err := testDB.QueryRow(t.Context(), `
		SELECT title FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Buy milk'
		LIMIT 1`, userID,
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Buy milk", title)
}

func TestEditTask(t *testing.T) {
	id := createTask(t, "Task to edit")

	editForm := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/"+id+"/edit",
	)
	rs := editForm.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)

	update := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/edit",
	)
	update.SetContentType(test.FormContentType)
	update.SetFollowRedirect(false)
	//nolint:exhaustruct // only Title needed for this test
	update.SetData(dtos.SaveTaskDto{Title: "Updated title"})
	rs = update.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var title string
	err := testDB.QueryRow(t.Context(),
		`SELECT title FROM todos.tasks WHERE id = $1`, id,
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", title)
}

func TestSettings(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/settings")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSettingsLabels(t *testing.T) {
	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddLabelPresetDto{Category: "setup", Value: "DM-Single1"})
	rs := add.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var count int
	err := testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.label_presets
		WHERE user_id = $1 AND category = 'setup' AND value = 'DM-Single1'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	remove := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels/setup/DM-Single1/delete",
	)
	remove.SetFollowRedirect(false)
	rs = remove.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.label_presets
		WHERE user_id = $1 AND category = 'setup' AND value = 'DM-Single1'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSettingsURLPatterns(t *testing.T) {
	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/url-patterns",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://jira.example.com/browse/",
		PlatformName: "Jira",
		TypeLabel:    "CR",
	})
	rs := add.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.url_patterns WHERE user_id = $1 LIMIT 1`, userID,
	).Scan(&id)
	require.NoError(t, err)

	remove := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/url-patterns/"+id+"/delete",
	)
	remove.SetFollowRedirect(false)
	rs = remove.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestSettingsLabels_InvalidCategory(t *testing.T) {
	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddLabelPresetDto{Category: "invalid", Value: "X"})
	rs := add.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

func TestSettingsArchive(t *testing.T) {
	update := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/archive",
	)
	update.SetContentType(test.FormContentType)
	update.SetFollowRedirect(false)
	update.SetData(dtos.UpdateArchiveDto{ArchiveAfterHours: 48})
	rs := update.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var hours int
	err := testDB.QueryRow(t.Context(), `
		SELECT archive_after_hours FROM todos.archive_settings WHERE user_id = $1`, userID,
	).Scan(&hours)
	require.NoError(t, err)
	assert.Equal(t, 48, hours)
}
