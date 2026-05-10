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
	tReq.SetData(dtos.QuickAddDto{Input: "Buy milk", Description: "", SectionID: ""})
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

func TestQuickAdd_UsesHiddenSectionID(t *testing.T) {
	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'Quick Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Task in section",
		Description: "",
		SectionID:   sectionID,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var gotSectionID *string
	err = testDB.QueryRow(t.Context(), `
		SELECT section_id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Task in section'
		LIMIT 1`, userID,
	).Scan(&gotSectionID)
	require.NoError(t, err)
	require.NotNil(t, gotSectionID)
	assert.Equal(t, sectionID, *gotSectionID)
}

func TestMoveTaskSection(t *testing.T) {
	id := createTask(t, "Task to move")

	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'Move Target') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	move := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/section",
	)
	move.SetContentType(test.FormContentType)
	move.SetFollowRedirect(false)
	move.SetData(dtos.MoveSectionDto{SectionID: sectionID})
	rs := move.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var gotSectionID *string
	err = testDB.QueryRow(t.Context(),
		`SELECT section_id::text FROM todos.tasks WHERE id = $1`, id,
	).Scan(&gotSectionID)
	require.NoError(t, err)
	require.NotNil(t, gotSectionID)
	assert.Equal(t, sectionID, *gotSectionID)

	// Move back to no section.
	clearMove := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/section",
	)
	clearMove.SetContentType(test.FormContentType)
	clearMove.SetFollowRedirect(false)
	clearMove.SetData(dtos.MoveSectionDto{SectionID: ""})
	rs = clearMove.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	err = testDB.QueryRow(t.Context(),
		`SELECT section_id::text FROM todos.tasks WHERE id = $1`, id,
	).Scan(&gotSectionID)
	require.NoError(t, err)
	assert.Nil(t, gotSectionID)
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

func TestQuickUpdate_ChangesTitle(t *testing.T) {
	id := createTask(t, "Original title")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/quick-update",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(
		dtos.QuickAddDto{Input: "Updated title p2", Description: "", SectionID: ""},
	)
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var title string
	var priority int
	err := testDB.QueryRow(t.Context(),
		`SELECT title, priority FROM todos.tasks WHERE id = $1`, id,
	).Scan(&title, &priority)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", title)
	assert.Equal(t, 2, priority)
}

func TestQuickUpdate_ISODueDate(t *testing.T) {
	id := createTask(t, "Task with date")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/quick-update",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input: "Task with date 2026-06-01", Description: "", SectionID: "",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var due *string
	err := testDB.QueryRow(t.Context(),
		`SELECT due_date::text FROM todos.tasks WHERE id = $1`, id,
	).Scan(&due)
	require.NoError(t, err)
	require.NotNil(t, due)
	assert.Equal(t, "2026-06-01", *due)
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
	add.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "DM-Single1"})
	rs := add.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var count int
	err := testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.label_presets
		WHERE user_id = $1 AND category = 'label' AND value = 'DM-Single1'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	remove := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels/label/DM-Single1/delete",
	)
	remove.SetFollowRedirect(false)
	rs = remove.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.label_presets
		WHERE user_id = $1 AND category = 'label' AND value = 'DM-Single1'`, userID,
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
		Label:        "CR",
		Shortcut:     "",
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
