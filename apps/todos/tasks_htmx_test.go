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

// ── Reorder tasks ─────────────────────────────────────────────────────────────

func TestReorderTasks_Success(t *testing.T) {
	id1 := createTask(t, "Reorder task 1")
	id2 := createTask(t, "Reorder task 2")

	body := strings.NewReader(`{"ids":["` + id2 + `","` + id1 + `"]}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/reorder", body)
	req.Header.Set("Content-Type", test.JSONContentType)

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestReorderTasks_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not json`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/reorder", body)
	req.Header.Set("Content-Type", test.JSONContentType)

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ── Search handler ────────────────────────────────────────────────────────────

func TestSearchHandler_ReturnsPage(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/search")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestSearchHandler_WithQuery(t *testing.T) {
	uniqueTitle := "UniqueSearchableTitle99887766"
	_ = createTask(t, uniqueTitle)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/search?q="+uniqueTitle,
	)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// ── UpdateSubtask invalid IDs ─────────────────────────────────────────────────

func TestUpdateSubtask_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/not-a-uuid/subtasks/not-a-uuid/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestUpdateSubtask_InvalidSubtaskID(t *testing.T) {
	taskID := createTask(t, "Task for subtask update test")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/"+taskID+"/subtasks/not-a-uuid/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// ── QuickAdd HTMX ─────────────────────────────────────────────────────────────

func TestQuickAdd_HTMX(t *testing.T) {
	body := strings.NewReader("input=HTMX+task&description=&section_id=")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/", body)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestQuickAdd_EmptyInput(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{Input: "", Description: "", SectionID: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// ── QuickUpdate HTMX ──────────────────────────────────────────────────────────

func TestQuickUpdate_HTMX(t *testing.T) {
	id := createTask(t, "HTMX quick update task")

	body := strings.NewReader("input=Updated+title&description=&section_id=")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/quick-update", body,
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestQuickUpdate_HTMX_WithSection(t *testing.T) {
	id := createTask(t, "HTMX quick update task with section")

	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'QU Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	body := strings.NewReader(
		"input=Updated+with+section&description=&section_id=" + sectionID,
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/todos/"+id+"/quick-update?section="+sectionID,
		body,
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── MoveSection HTMX ──────────────────────────────────────────────────────────

func TestMoveSection_HTMX(t *testing.T) {
	id := createTask(t, "HTMX move section task")

	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'HTMX Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	body := strings.NewReader("section_id=" + sectionID)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/section", body,
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMoveSection_HTMX_WithCurrentSection(t *testing.T) {
	id := createTask(t, "HTMX move section with current")

	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'Current Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	body := strings.NewReader("section_id=")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/todos/"+id+"/section?current="+sectionID,
		body,
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMoveSection_ToNoSection_HTMX(t *testing.T) {
	id := createTask(t, "HTMX move to no section task")

	body := strings.NewReader("section_id=")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/section", body,
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── UpdateSubtask happy path ───────────────────────────────────────────────────

func TestUpdateSubtask_Success(t *testing.T) {
	taskID := createTask(t, "Task for subtask update")

	// Create subtask via POST
	addReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/subtasks",
	)
	addReq.SetContentType(test.FormContentType)
	addReq.SetFollowRedirect(false)
	addReq.SetData(dtos.AddSubtaskDto{
		Input:           "Initial subtask",
		Description:     "",
		Source:          "",
		ParentSubtaskID: "",
	})
	rs := addReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var subID string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.subtasks
		WHERE task_id = $1 AND title = 'Initial subtask'`, taskID,
	).Scan(&subID)
	require.NoError(t, err)

	// Update it
	editReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/"+taskID+"/subtasks/"+subID+"/edit",
	)
	editReq.SetContentType(test.FormContentType)
	editReq.SetFollowRedirect(false)
	editReq.SetData(dtos.UpdateSubtaskDto{
		Title:       "Updated subtask",
		Description: "",
		Priority:    0,
		Label:       "",
		DueDate:     "",
		Deadline:    "",
	})
	rs = editReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var title string
	err = testDB.QueryRow(t.Context(), `
		SELECT title FROM todos.subtasks WHERE id = $1`, subID,
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Updated subtask", title)
}

// ── listTasksHandler HTMX with tasks ─────────────────────────────────────────

func TestListTasks_HTMX_WithTasks(t *testing.T) {
	createTask(t, "Task for HTMX list test")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/", nil)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── listTasksHandler with section param ───────────────────────────────────────

func TestListTasks_WithSection(t *testing.T) {
	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'List Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet, "/todos/?w=private&section="+sectionID, nil,
	)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
