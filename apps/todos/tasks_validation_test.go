package todos_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
)

// ── resolveSection (via HTMX quick-add) ──────────────────────────────────────

func TestQuickAdd_HTMX_WithValidSection(t *testing.T) {
	// Insert a real section so resolveSection can find it.
	var sectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'HTMX Section') RETURNING id::text`, userID,
	).Scan(&sectionID)
	require.NoError(t, err)

	body := strings.NewReader(
		"input=HTMX+section+task&description=&section_id=" + sectionID,
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/", body)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestQuickAdd_HTMX_WithInvalidSectionUUID(t *testing.T) {
	// "not-a-uuid" hits the uuid.Parse error branch of resolveSection → nil section.
	body := strings.NewReader(
		"input=HTMX+bad+uuid+task&description=&section_id=not-a-uuid",
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/", body)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestQuickAdd_HTMX_WithUnknownSectionUUID(t *testing.T) {
	// Create a real section to anchor the task, but pass a *different*
	// valid UUID as section_id in the form — resolveSection will parse
	// the UUID fine but not find it in the list (returns nil).
	var anchorSectionID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'Anchor Section') RETURNING id::text`, userID,
	).Scan(&anchorSectionID)
	require.NoError(t, err)

	// The input has "#Anchor Section" so QuickAdd uses it as the task section,
	// while the form's section_id is an unrelated UUID that will not be found.
	body := strings.NewReader(
		"input=HTMX+task+%23Anchor+Section&description=" +
			"&section_id=00000000-dead-beef-0000-000000000001",
	)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/", body)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── safeLocalRedirectTarget (via non-HTMX completeTaskHandler) ───────────────

func TestCompleteTask_NonHTMX_WithLocalBack(t *testing.T) {
	id := createTask(t, "Task complete local back")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/complete?back=/todos/done", nil,
	)
	getRoutes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/todos/done", rr.Header().Get("Location"))
}

func TestCompleteTask_NonHTMX_WithExternalBack(t *testing.T) {
	id := createTask(t, "Task complete external back")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/todos/"+id+"/complete?back=https://evil.com/steal",
		nil,
	)
	getRoutes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	// External URL must be blocked → falls back to todosRoot.
	assert.Equal(t, "/todos/", rr.Header().Get("Location"))
}

func TestCompleteTask_NonHTMX_WithRelativeBack(t *testing.T) {
	id := createTask(t, "Task complete relative back")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/complete?back=relative", nil,
	)
	getRoutes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	// Relative path without leading "/" → blocked.
	assert.Equal(t, "/todos/", rr.Header().Get("Location"))
}

// ── ensureSections (via quick-add with #tag in title) ────────────────────────

func TestQuickAdd_WithHashSection(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Buy groceries #Shopping",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// ensureSections should have created the "Shopping" section.
	var count int
	err := testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.sections
		WHERE owner_user_id = $1 AND name = 'Shopping'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQuickAdd_WithExistingHashSection(t *testing.T) {
	// Pre-create the section so ensureSections hits the "already exists" branch.
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.sections (owner_user_id, name)
		VALUES ($1, 'Existing')`, userID)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Do something #Existing",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Should still have exactly one "Existing" section (no duplicate created).
	var count int
	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.sections
		WHERE owner_user_id = $1 AND name = 'Existing'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// ── enrichWithShortcuts (via GET /todos/ with URL pattern + link) ─────────────

func TestListTasks_WithShortcutBadge(t *testing.T) {
	// 1. Add URL pattern with shortcut.
	addPattern := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/url-patterns",
	)
	addPattern.SetContentType(test.FormContentType)
	addPattern.SetFollowRedirect(false)
	addPattern.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://badge.example.com/",
		PlatformName: "Badge",
		Label:        "B",
		Shortcut:     "BADGE",
	})
	rs := addPattern.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// 2. Create a task with a link matching the pattern.
	newTask := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/new",
	)
	newTask.SetContentType(test.FormContentType)
	newTask.SetFollowRedirect(false)
	newTask.SetData(dtos.SaveTaskDto{ //nolint:exhaustruct // partial
		Title:    "Badge task",
		LinkURLs: []string{"https://badge.example.com/123"},
	})
	rs = newTask.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// 3. List open tasks — enrichWithShortcuts fills ShortcutBadge when patterns exist.
	// Use ?w=private to skip the workspace-redirect logic in applyWorkspaceParam.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── listTasksHandler / applyWorkspaceParam branches ──────────────────────────

// TestListTasks_HXRequestNoW exercises applyWorkspaceParam's "HX-Request + no
// ?w= parameter → no redirect, serve partial" path.
func TestListTasks_HXRequestNoW(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/", nil)
	req.Header.Set("HX-Request", "true")
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestListTasks_InvalidSectionUUID covers the uuid.Parse-error branch when
// ?section= contains a non-UUID value → 404.
func TestListTasks_InvalidSectionUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/")
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"w": {"private"}, "section": {"not-a-uuid"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// TestListTasks_UnknownSectionUUID covers the "section UUID valid but not found"
// branch → 404.
func TestListTasks_UnknownSectionUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/")
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{
		"w":       {"private"},
		"section": {"00000000-0000-0000-0000-000000000000"},
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// TestListTasks_InvalidWorkspaceUUID covers the applyWorkspaceParam path where
// ?w= is neither "private" nor a valid UUID → treated as nil (private mode).
func TestListTasks_InvalidWorkspaceUUID(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=not-a-uuid", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── QuickAdd: fancy URL / deadline / recurrence parsing branches ──────────────

// TestQuickAdd_FancyURL exercises the parseFancyURL branch inside QuickAdd when
// the input uses Edge-style Markdown link format: [Title](https://…).
func TestQuickAdd_FancyURL(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "[Buy groceries](https://example.com/shopping)",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickAdd_DeadlineNextWeekday covers the "!next <weekday>" branch inside
// parseDeadlineTok — the skip=1 / "next" keyword path.
func TestQuickAdd_DeadlineNextWeekday(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Dentist appointment !next monday",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickAdd_DeadlineISODate covers the time.Parse("2006-01-02") fallback
// inside parseDeadlineTok when the token is a bare ISO date.
func TestQuickAdd_DeadlineISODate(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "File taxes !2026-04-15",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickAdd_RecurEveryNDays covers the "every N days" branch in parseEveryDate.
func TestQuickAdd_RecurEveryNDays(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Water plants every 3 days",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickAdd_RecurEveryFirstMonday covers the "every ordinal weekday" branch
// in parseEveryDate (wdOK && ordinalWord != "").
func TestQuickAdd_RecurEveryFirstMonday(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Monthly review every first monday",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickUpdate_EmptyInputKeepsTitle exercises the "title == "" fallback
// inside QuickUpdate that preserves the existing title.
func TestQuickUpdate_EmptyInputKeepsTitle(t *testing.T) {
	id := createTask(t, "Keep this title")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/quick-update",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{Input: "   ", Description: "", SectionID: ""})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var title string
	err := testDB.QueryRow(t.Context(),
		`SELECT title FROM todos.tasks WHERE id = $1`, id,
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Keep this title", title)
}

// TestQuickUpdate_WithRecurrence exercises the recur parsing branches inside
// QuickUpdate (parsedDTO.Recur != "" → parseRecurOnly).
func TestQuickUpdate_WithRecurrence(t *testing.T) {
	id := createTask(t, "Recurring task to update")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/quick-update",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{
		Input:       "Weekly standup every monday",
		Description: "",
		SectionID:   "",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestQuickUpdate_InvalidUUID covers the uuid.Parse error branch in
// quickUpdateHandler.
func TestQuickUpdate_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/not-a-uuid/quick-update",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.QuickAddDto{Input: "anything", Description: "", SectionID: ""})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// ── completeTaskHandler / deleteTaskHandler HTMX + invalid-UUID branches ─────

func TestCompleteTask_HTMX(t *testing.T) {
	id := createTask(t, "HTMX complete task")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/"+id+"/complete", nil)
	req.Header.Set("HX-Request", "true")
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCompleteTask_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/not-a-uuid/complete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestCompleteTask_WithBack(t *testing.T) {
	id := createTask(t, "Complete with back task")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"back": {"/todos"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos", rs.Header.Get("Location"))
}

func TestDeleteTask_HTMX(t *testing.T) {
	id := createTask(t, "HTMX delete task")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/todos/"+id+"/delete", nil)
	req.Header.Set("HX-Request", "true")
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteTask_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestDeleteTask_WithRelativeBack(t *testing.T) {
	id := createTask(t, "Delete with relative back")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/delete",
	)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"back": {"/todos"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos", rs.Header.Get("Location"))
}

func TestDeleteTask_WithNoSlashBack(t *testing.T) {
	id := createTask(t, "Delete no-slash back")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/delete",
	)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"back": {"todos-relative"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos/", rs.Header.Get("Location"))
}

// ── quickAddHandler back-URL redirect tests ───────────────────────────────────

// TestQuickAdd_WithRelativeBack exercises the `back != ""` else-branch where
// back is a valid relative URL and is used as the redirect target.
func TestQuickAdd_WithRelativeBack(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(
		dtos.QuickAddDto{Input: "Back URL task", Description: "", SectionID: ""},
	)
	tReq.SetQuery(url.Values{"back": {"/todos"}})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos", rs.Header.Get("Location"))
}

// TestQuickAdd_WithAbsoluteBack verifies that absolute URLs in ?back= are
// rejected (security: prevent open redirects) and the user is sent to todosRoot.
func TestQuickAdd_WithAbsoluteBack(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(
		dtos.QuickAddDto{Input: "Absolute back task", Description: "", SectionID: ""},
	)
	tReq.SetQuery(url.Values{"back": {"https://evil.com/steal"}})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.NotEqual(t, "https://evil.com/steal", rs.Header.Get("Location"))
}

// ── createTaskHandler service-error branch coverage ──────────────────────────

// TestCreateTask_InvalidDueDate exercises the Tasks.Create error path in
// createTaskHandler by supplying an unparseable due date → 400.
func TestCreateTask_InvalidDueDate(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct // only Title and DueDate needed for this test
	tReq.SetData(dtos.SaveTaskDto{
		Title:   "Task with bad date",
		DueDate: "xyz-invalid-date-string",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── editTaskFormHandler UUID-error branch coverage ────────────────────────────

// TestEditTaskForm_InvalidUUID exercises the uuid.Parse-error branch in
// editTaskFormHandler → 404.
func TestEditTaskForm_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/bad-uuid/edit",
	)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// ── updateTaskHandler service-error branch coverage ───────────────────────────

// TestUpdateTask_InvalidDueDate exercises the Tasks.Update error path in
// updateTaskHandler by supplying an unparseable due date → 400.
func TestUpdateTask_InvalidDueDate(t *testing.T) {
	taskID := createTask(t, "Task for bad due date update")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct // only Title and DueDate needed for this test
	tReq.SetData(dtos.SaveTaskDto{
		Title:   "Task for bad due date update",
		DueDate: "xyz-not-a-date",
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// ── safeBackRedirect no-leading-slash branch coverage ─────────────────────────

// TestCompleteTask_NoLeadingSlashBack exercises the !HasPrefix("/") branch in
// safeBackRedirect → redirects to todosRoot instead of the relative path.
func TestCompleteTask_NoLeadingSlashBack(t *testing.T) {
	taskID := createTask(t, "Task for no-slash back test")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/complete",
	)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"back": {"todos/done"}}) // no leading slash
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos/", rs.Header.Get("Location"))
}

// ── updateTaskHandler additional branch coverage ──────────────────────────────

// TestUpdateTask_XAutoSave covers the X-Auto-Save: 1 header path in
// updateTaskHandler which returns 204 NoContent without redirecting.
func TestUpdateTask_XAutoSave(t *testing.T) {
	taskID := createTask(t, "Auto-save task")

	body := strings.NewReader("title=Auto-save+task&description=&due_date=&priority=0")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/todos/"+taskID+"/edit",
		body,
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Auto-Save", "1")
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestUpdateTask_WithBack covers the ?back= redirect param in updateTaskHandler.
func TestUpdateTask_WithBack(t *testing.T) {
	taskID := createTask(t, "Back-param task")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/edit",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"back": {"/todos/"}})
	tReq.SetData(dtos.SaveTaskDto{Title: "Back-param task"}) //nolint:exhaustruct // ok
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/todos/", rs.Header.Get("Location"))
}

// ── handleSubtaskAction invalid-UUID branch coverage ─────────────────────────

// TestToggleSubtask_InvalidTaskUUID exercises the taskID UUID-parse-error branch
// in handleSubtaskAction → 404.
func TestToggleSubtask_InvalidTaskUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/bad-task-id/subtasks/00000000-0000-0000-0000-000000000000/toggle",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// TestToggleSubtask_InvalidSubtaskUUID exercises the sid UUID-parse-error branch
// in handleSubtaskAction → 404.
func TestToggleSubtask_InvalidSubtaskUUID(t *testing.T) {
	taskID := createTask(t, "Task for invalid subtask UUID test")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/"+taskID+"/subtasks/bad-subtask-id/toggle",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// ── reopenTaskHandler invalid-UUID branch coverage ───────────────────────────

// TestReopenTask_InvalidUUID covers the UUID-parse-error branch in
// reopenTaskHandler → 404.
func TestReopenTask_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/bad-id/reopen",
	)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// ── moveSectionHandler additional coverage ────────────────────────────────────

// TestMoveSection_InvalidTaskUUID covers the UUID-parse-error branch in
// moveSectionHandler → 404.
func TestMoveSection_InvalidTaskUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/bad-id/section",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.MoveSectionDto{SectionID: ""})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

// TestMoveSection_NonHTMX exercises the non-HTMX redirect path in
// moveSectionHandler (no HX-Request header → redirect to ?back= or todosRoot).
func TestMoveSection_NonHTMX(t *testing.T) {
	taskID := createTask(t, "Task for non-HTMX move section")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/section",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.MoveSectionDto{SectionID: ""})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}
