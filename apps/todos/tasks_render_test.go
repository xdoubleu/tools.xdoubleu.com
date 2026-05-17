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

// ── Rich task helper ──────────────────────────────────────────────────────────

func createRichTask(t *testing.T, title string) string {
	t.Helper()
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SaveTaskDto{
		Title:       title,
		Description: "A detailed description",
		Label:       "DM-Single",
		DueDate:     "2026-06-01",
		Deadline:    "2026-07-01",
		Priority:    2,
		SectionID:   "",
		Recur:       "",
		RecurDays:   0,
		LinkURLs:    []string{"https://example.com/ticket/123"},
		LinkLabels:  []string{"JIRA-123"},
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = $2`, userID, title,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// ── searchHandler with results ────────────────────────────────────────────────

func TestSearch_WithResults(t *testing.T) {
	createTask(t, "Unique searchable task XYZ")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/search?q=XYZ",
	)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSearch_WithDoneTask creates and completes a task then searches for it,
// covering the case models.StatusDone branch in searchHandler.
func TestSearch_WithDoneTask(t *testing.T) {
	id := createTask(t, "SearchDone_unique_task_ZZQ")

	done := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	done.SetFollowRedirect(false)
	require.Equal(t, http.StatusSeeOther, done.Do(t).StatusCode)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/search",
	)
	tReq.SetQuery(url.Values{"q": {"SearchDone_unique_task_ZZQ"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSearch_WithArchivedTask inserts a task with status "archive" directly
// so searchHandler hits the default case in the status switch.
func TestSearch_WithArchivedTask(t *testing.T) {
	var taskID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status)
		VALUES ($1, 'SearchArchived_unique_ZZR', 'archived')
		RETURNING id::text`, userID,
	).Scan(&taskID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(t.Context(), `DELETE FROM todos.tasks WHERE id = $1`, taskID)
	})

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodGet, "/todos/search",
	)
	tReq.SetQuery(url.Values{"q": {"SearchArchived_unique_ZZR"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// ── Rich task render tests ────────────────────────────────────────────────────

func TestListTasks_WithRichData(t *testing.T) {
	createRichTask(t, "Rich task for list coverage")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListTasks_HTMX_WithRichData(t *testing.T) {
	createRichTask(t, "Rich task for HTMX list coverage")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/", nil)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestViewTask_WithRichData(t *testing.T) {
	id := createRichTask(t, "Rich task for view coverage")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestViewTask_WithSubtasks(t *testing.T) {
	id := createRichTask(t, "Rich task with subtasks")

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.subtasks
			(task_id, title, priority, due_date, deadline)
		VALUES ($1, 'Subtask with data', 1, '2026-06-15', '2026-07-15')`,
		id,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestQuickUpdate_HTMX_WithRichTask(t *testing.T) {
	id := createRichTask(t, "Rich task for quick update HTMX")

	body := "input=Updated+rich+task&description=desc"
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/todos/"+id+"/quick-update",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", test.FormContentType)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── Done and archive pages with tasks ─────────────────────────────────────────

func TestListDone_WithTasks(t *testing.T) {
	id := createRichTask(t, "Task to complete for done page")

	cReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	cReq.SetFollowRedirect(false)
	rs := cReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/done")
	rs = tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestListDone_HTMX_WithTasks(t *testing.T) {
	id := createRichTask(t, "Task to complete for done HTMX")

	cReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	cReq.SetFollowRedirect(false)
	rs := cReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/done", nil)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListArchive_WithTasks(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status)
		VALUES ($1, 'Archived task for archive page', 'archived')`, userID,
	)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/todos/archive")
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestListArchive_HTMX_WithTasks(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status)
		VALUES ($1, 'Archived task HTMX', 'archived')`, userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/archive", nil)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSearch_HTMX_WithResults(t *testing.T) {
	createTask(t, "HTMX searchable task QWERTY")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/search?q=QWERTY", nil)
	req.Header.Set("HX-Request", "true")

	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── Recurring task rendering ───────────────────────────────────────────────────

func TestViewTask_WithRecurRule(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SaveTaskDto{
		Title:       "Recurring task for coverage",
		Description: "",
		Label:       "",
		DueDate:     "2026-06-01",
		Deadline:    "",
		Priority:    0,
		SectionID:   "",
		Recur:       "7",
		RecurDays:   0,
		LinkURLs:    nil,
		LinkLabels:  nil,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Recurring task for coverage'`, userID,
	).Scan(&id)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestViewTask_WithRecurDays(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SaveTaskDto{
		Title:       "Every N days task",
		Description: "",
		Label:       "",
		DueDate:     "2026-06-01",
		Deadline:    "",
		Priority:    1,
		SectionID:   "",
		Recur:       "",
		RecurDays:   7,
		LinkURLs:    nil,
		LinkLabels:  nil,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Every N days task'`, userID,
	).Scan(&id)
	require.NoError(t, err)

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

// ── Settings page with data ────────────────────────────────────────────────────

func TestSettingsPage_WithData(t *testing.T) {
	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddLabelPresetDto{Category: "label", Value: "Coverage-Label"})
	rs := add.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/settings", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── Priority badge level 3 ────────────────────────────────────────────────────

func TestListTasks_Priority3(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SaveTaskDto{
		Title:       "P3 priority task",
		Description: "",
		Label:       "",
		DueDate:     "",
		Deadline:    "",
		Priority:    3,
		SectionID:   "",
		Recur:       "",
		RecurDays:   0,
		LinkURLs:    nil,
		LinkLabels:  nil,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ── Recurring task: Complete creates a new instance ───────────────────────────

func TestCompleteRecurringTask(t *testing.T) {
	title := "RecurTestEveryThursday"

	// Insert a task with recur_rule set directly so the Complete branch fires.
	// Thursday is weekday 4 in Go's time.Weekday numbering.
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks
			(owner_user_id, title, recur_rule, recur_days, due_date)
		VALUES ($1, $2, 'weekday:4', 0, CURRENT_DATE)`,
		userID, title,
	)
	require.NoError(t, err)

	var id string
	err = testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = $2
		ORDER BY created_at DESC LIMIT 1`,
		userID, title,
	).Scan(&id)
	require.NoError(t, err)

	complete := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	complete.SetFollowRedirect(false)
	rs := complete.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Original task should be done.
	var status string
	err = testDB.QueryRow(t.Context(),
		`SELECT status FROM todos.tasks WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	// A new recurring instance should have been created.
	var count int
	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.tasks
		WHERE owner_user_id = $1 AND title = $2 AND status = 'open'`,
		userID, title,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected a new open recurring instance to be created")
}

func TestCompleteRecurringTask_WithLinks(t *testing.T) {
	title := "RecurTestWithLinks"

	// Insert recurring task.
	var id string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks
			(owner_user_id, title, recur_rule, recur_days, due_date)
		VALUES ($1, $2, 'days:7', 7, CURRENT_DATE)
		RETURNING id::text`,
		userID, title,
	).Scan(&id)
	require.NoError(t, err)

	// Add a link to the task.
	_, err = testDB.Exec(t.Context(), `
		INSERT INTO todos.task_links (task_id, url, label, sort_order)
		VALUES ($1::uuid, 'https://example.com/task/99', 'EX-99', 0)`,
		id,
	)
	require.NoError(t, err)

	complete := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	complete.SetFollowRedirect(false)
	rs := complete.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Original task should be done.
	var status string
	err = testDB.QueryRow(t.Context(),
		`SELECT status FROM todos.tasks WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	// A new instance must exist with the same link URL.
	var newTaskID string
	err = testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = $2 AND status = 'open'
		ORDER BY created_at DESC LIMIT 1`,
		userID, title,
	).Scan(&newTaskID)
	require.NoError(t, err)
	assert.NotEmpty(t, newTaskID)

	var linkCount int
	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.task_links
		WHERE task_id = $1::uuid AND url = 'https://example.com/task/99'`,
		newTaskID,
	).Scan(&linkCount)
	require.NoError(t, err)
	assert.Equal(t, 1, linkCount, "new recurring instance should carry the link")
}

// ── Search by URL shortcut → SearchByLinkURL ──────────────────────────────────

func TestSearch_ByURLShortcut(t *testing.T) {
	// 1. Add a URL pattern with a shortcut.
	addPattern := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/url-patterns",
	)
	addPattern.SetContentType(test.FormContentType)
	addPattern.SetFollowRedirect(false)
	addPattern.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://example.com/issue/",
		PlatformName: "PROJ",
		Label:        "PROJ",
		Shortcut:     "PROJ",
	})
	rs := addPattern.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Retrieve the pattern ID for cleanup.
	var patternID string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.url_patterns
		WHERE user_id = $1 AND shortcut = 'PROJ'
		LIMIT 1`,
		userID,
	).Scan(&patternID)
	require.NoError(t, err)

	// 2. Create a task via quick-add using the matching URL.
	taskTitle := "https://example.com/issue/42"
	addTask := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	addTask.SetContentType(test.FormContentType)
	addTask.SetFollowRedirect(false)
	addTask.SetData(dtos.QuickAddDto{Input: taskTitle, Description: "", SectionID: ""})
	rs = addTask.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Verify the task was stored with a link to that URL.
	var taskID string
	err = testDB.QueryRow(t.Context(), `
		SELECT t.id::text FROM todos.tasks t
		JOIN todos.task_links l ON l.task_id = t.id
		WHERE t.owner_user_id = $1 AND l.url = 'https://example.com/issue/42'
		LIMIT 1`,
		userID,
	).Scan(&taskID)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// 3. Search by shortcut "PROJ42" — should route through searchByShortcut
	//    and call SearchByLinkURL which returns the task above.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/search?q=PROJ42", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "example.com/issue/42")

	// Cleanup: remove the URL pattern.
	del := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/todos/settings/url-patterns/"+patternID+"/delete",
	)
	del.SetFollowRedirect(false)
	_ = del.Do(t)
}

// ── Archive search by URL shortcut → TaskService.Search ok=true ──────────────

// TestArchive_ByShortcut exercises the `ok=true` branch of TaskService.Search
// (via listArchiveHandler), covering the `return tasks, err` inside the
// searchByShortcut guard.
func TestArchive_ByShortcut(t *testing.T) {
	// Add a URL pattern.
	addPattern := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/url-patterns",
	)
	addPattern.SetContentType(test.FormContentType)
	addPattern.SetFollowRedirect(false)
	addPattern.SetData(dtos.AddURLPatternDto{
		URLPrefix:    "https://archive-search.example.com/issue/",
		PlatformName: "ARCH",
		Label:        "ARCH",
		Shortcut:     "ARCH",
	})
	require.Equal(t, http.StatusSeeOther, addPattern.Do(t).StatusCode)

	var patternID string
	err := testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.url_patterns
		WHERE user_id = $1 AND shortcut = 'ARCH'
		LIMIT 1`, userID,
	).Scan(&patternID)
	require.NoError(t, err)

	// Create a task via quick-add with a matching link URL.
	addTask := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/")
	addTask.SetContentType(test.FormContentType)
	addTask.SetFollowRedirect(false)
	addTask.SetData(dtos.QuickAddDto{
		Input:       "https://archive-search.example.com/issue/99",
		Description: "",
		SectionID:   "",
	})
	require.Equal(t, http.StatusSeeOther, addTask.Do(t).StatusCode)

	// Hit /todos/archive with a shortcut query to exercise TaskService.Search ok=true.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/archive?q=ARCH99", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Cleanup the URL pattern.
	del := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/url-patterns/"+patternID+"/delete",
	)
	del.SetFollowRedirect(false)
	_ = del.Do(t)
}

// ── Colored label badge coverage ──────────────────────────────────────────────

// TestListTasks_WithColoredLabel adds a label preset with a color, creates a
// task with that label, then renders the list page to exercise the colored
// badge branch in the labelBadges template.
func TestListTasks_WithColoredLabel(t *testing.T) {
	const labelVal = "ColoredCovLbl"

	// Add the label preset.
	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddLabelPresetDto{Category: "label", Value: labelVal})
	require.Equal(t, http.StatusSeeOther, add.Do(t).StatusCode)

	// Set a color for it.
	setColor := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels/label/"+labelVal+"/color",
	)
	setColor.SetContentType(test.FormContentType)
	setColor.SetFollowRedirect(false)
	setColor.SetData(dtos.UpdateLabelColorDto{Color: "#ff0000"})
	require.Equal(t, http.StatusSeeOther, setColor.Do(t).StatusCode)

	// Create a task with this colored label.
	tCreate := test.CreateRequestTester(getRoutes(), http.MethodPost, "/todos/new")
	tCreate.SetContentType(test.FormContentType)
	tCreate.SetFollowRedirect(false)
	tCreate.SetData(dtos.SaveTaskDto{ //nolint:exhaustruct // partial
		Title: "Task with colored label for coverage",
		Label: labelVal,
	})
	require.Equal(t, http.StatusSeeOther, tCreate.Do(t).StatusCode)

	// Render the list page so the colored badge branch fires.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "#ff0000")
}

// TestArchivePage_WithColoredLabelTask inserts a task with a colored label
// into archived status and renders the archive page.
func TestArchivePage_WithColoredLabelTask(t *testing.T) {
	const lblVal = "ArchColorLbl"

	add := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/labels",
	)
	add.SetContentType(test.FormContentType)
	add.SetFollowRedirect(false)
	add.SetData(dtos.AddLabelPresetDto{Category: "label", Value: lblVal})
	_ = add.Do(t)

	setColor := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/todos/settings/labels/label/"+lblVal+"/color",
	)
	setColor.SetContentType(test.FormContentType)
	setColor.SetFollowRedirect(false)
	setColor.SetData(dtos.UpdateLabelColorDto{Color: "#00ff00"})
	_ = setColor.Do(t)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, labels)
		VALUES ($1, 'Archived colored label task', 'archived', ARRAY[$2])`,
		userID, lblVal,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/archive", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestListTasks_Priority1 covers the P1 branch in the priorityBadge template.
func TestListTasks_Priority1(t *testing.T) {
	tCreate := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/",
	)
	tCreate.SetContentType(test.FormContentType)
	tCreate.SetFollowRedirect(false)
	tCreate.SetData(dtos.SaveTaskDto{ //nolint:exhaustruct // partial
		Title:    "Priority 1 task",
		Priority: 1,
	})
	require.Equal(t, http.StatusSeeOther, tCreate.Do(t).StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "P1")
}

// TestViewPage_DoneTask covers the "done" status branch in viewPageBody
// (Reopen button) and the CompletedAt badge branch.
func TestViewPage_DoneTask(t *testing.T) {
	id := createTask(t, "Task to complete for view")

	tComplete := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	tComplete.SetContentType(test.FormContentType)
	tComplete.SetFollowRedirect(false)
	require.Equal(t, http.StatusSeeOther, tComplete.Do(t).StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Reopen")
}

// TestViewPage_ArchivedTask covers the ArchivedAt branch in viewPageBody.
func TestViewPage_ArchivedTask(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, archived_at)
		VALUES ($1, 'Archived task for view', 'archived', NOW())`,
		userID,
	)
	require.NoError(t, err)

	var id string
	require.NoError(t, testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Archived task for view'
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestEditPage_DoneTask covers the "done" status branch in formPageBody
// (Reopen button shown in edit view).
func TestEditPage_DoneTask(t *testing.T) {
	id := createTask(t, "Task to complete for edit page")

	tComplete := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+id+"/complete",
	)
	tComplete.SetContentType(test.FormContentType)
	tComplete.SetFollowRedirect(false)
	require.Equal(t, http.StatusSeeOther, tComplete.Do(t).StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id+"/edit", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Reopen")
}

// TestEditPage_Priority1 covers the P1 selected branch in formPageBody.
func TestEditPage_Priority1(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, priority)
		VALUES ($1, 'P1 task for edit form coverage', 1)`,
		userID,
	)
	require.NoError(t, err)

	var id string
	require.NoError(t, testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'P1 task for edit form coverage'
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id+"/edit", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestEditPage_Priority3 covers the P3 selected branch in formPageBody.
func TestEditPage_Priority3(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, priority)
		VALUES ($1, 'P3 task for edit form coverage', 3)`,
		userID,
	)
	require.NoError(t, err)

	var id string
	require.NoError(t, testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'P3 task for edit form coverage'
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id+"/edit", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestArchivePage_QueryNoResults covers the "No tasks match" branch in
// archivePageBody when a query returns zero results.
func TestArchivePage_QueryNoResults(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet, "/todos/archive?q=xyznonexistentquery99zz", nil,
	)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestArchivePage_WithArchivedAt inserts a task with archived_at set so the
// archived date badge branch in archivePageBody is rendered.
func TestArchivePage_WithArchivedAt(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, archived_at)
		VALUES ($1, 'Archived with timestamp', 'archived', NOW())`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/archive", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestSearch_NoResults covers the "No tasks match" branch in searchPageBody
// when the query returns no results across all statuses.
func TestSearch_NoResults(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet, "/todos/search?q=xyznonexistentquery99zz", nil,
	)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestDonePage_WithCompletedAt covers the CompletedAt badge branch in
// donePageBody by inserting a done task with completed_at set.
func TestDonePage_WithCompletedAt(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, completed_at)
		VALUES ($1, 'Done task with completed_at', 'done', NOW())`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/done", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestEditPage_WithRichTask covers P2 selected, DueDate != nil, and
// Deadline != nil branches in formPageBody by viewing the edit form for a
// task that has all those fields populated.
func TestEditPage_WithRichTask(t *testing.T) {
	id := createRichTask(t, "Rich task for edit form coverage")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id+"/edit", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestDonePage_WithP1Task covers the P1 branch of priorityBadge when called
// from donePageBody.
func TestDonePage_WithP1Task(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, priority, completed_at)
		VALUES ($1, 'P1 done task', 'done', 1, NOW())`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/done", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "P1")
}

// TestArchivePage_WithP3Task covers the P3 branch of priorityBadge when
// called from archivePageBody.
func TestArchivePage_WithP3Task(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, priority, archived_at)
		VALUES ($1, 'P3 archived task', 'archived', 3, NOW())`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/archive", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "P3")
}

// TestViewPage_WithRecurDays covers the "else if RecurDays > 0" branch in
// viewPageBody for a task that has recur_days set but no recur_rule.
func TestViewPage_WithRecurDays(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, recur_days)
		VALUES ($1, 'Task with recur days only', 7)`,
		userID,
	)
	require.NoError(t, err)

	var id string
	require.NoError(t, testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 AND title = 'Task with recur days only'
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+id, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "every")
}

// TestListTasks_WithRecurDays covers the "else if RecurDays > 0" branch in
// taskRow (list page) by inserting a task with recur_days set.
func TestListTasks_WithRecurDays(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, recur_days)
		VALUES ($1, 'List recur days task', 5)`,
		userID,
	)
	require.NoError(t, err)

	// Use ?w=private to skip the workspace redirect and render the list directly.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "5d")
}

// TestViewTask_WithSubtaskDescription covers the "sub.Description != "" branch
// in subtaskItem by adding a subtask with a description and viewing the task.
func TestViewTask_WithSubtaskDescription(t *testing.T) {
	taskID := createTask(t, "Task for subtask description coverage")

	sub := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/subtasks",
	)
	sub.SetContentType(test.FormContentType)
	sub.SetFollowRedirect(false)
	sub.SetData(
		dtos.AddSubtaskDto{ //nolint:exhaustruct // only Input and Description needed
			Input:       "Subtask with description",
			Description: "This is the subtask description text",
		},
	)
	rs := sub.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+taskID, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "This is the subtask description text")
}

// TestListTasks_WithOnePolicy covers the else branch in listPageBody policies
// banner (exactly one policy → renders text directly, not a <ul>).
func TestListTasks_WithOnePolicy(t *testing.T) {
	// Clear any policies accumulated by earlier tests, then add exactly one.
	_, err := testDB.Exec(t.Context(),
		`DELETE FROM todos.policies WHERE owner_user_id = $1`, userID)
	require.NoError(t, err)

	policyReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/settings/policies",
	)
	policyReq.SetContentType(test.FormContentType)
	policyReq.SetFollowRedirect(false)
	policyReq.SetData(dtos.AddPolicyDto{
		Text:               "Single policy for list coverage",
		ReappearAfterHours: 24,
	})
	rs := policyReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Use ?w=private to skip the workspace redirect and render the list directly.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Single policy for list coverage")
}

// TestDonePage_WithDueDate covers the "task.DueDate != nil" branch in
// donePageBody by inserting a done task that has due_date set.
func TestDonePage_WithDueDate(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, due_date, completed_at)
		VALUES ($1, 'Done task with due date', 'done', NOW() + INTERVAL '7 days', NOW())`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/done", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestSearch_WithArchivedAt covers the "task.ArchivedAt != nil" branch in
// searchPageBody by inserting an archived task with archived_at set.
func TestSearch_WithArchivedAt(t *testing.T) {
	var taskID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, archived_at)
		VALUES ($1, 'SearchArchivedAt_unique_ZZQ', 'archived', NOW())
		RETURNING id::text`,
		userID,
	).Scan(&taskID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			t.Context(), `DELETE FROM todos.tasks WHERE id = $1`, taskID,
		)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet, "/todos/search?q=SearchArchivedAt_unique_ZZQ", nil,
	)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "archived")
}

// TestSearch_OpenTaskWithDueDate covers the "task.DueDate != nil" branch in
// searchPageBody Open section by inserting an open task with due_date set.
func TestSearch_OpenTaskWithDueDate(t *testing.T) {
	var taskID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, due_date)
		VALUES ($1, 'SearchDueDate_unique_ZZW', NOW() + INTERVAL '7 days')
		RETURNING id::text`,
		userID,
	).Scan(&taskID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(t.Context(), `DELETE FROM todos.tasks WHERE id = $1`, taskID)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet, "/todos/search?q=SearchDueDate_unique_ZZW", nil,
	)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "SearchDueDate_unique_ZZW")
}

// TestViewTask_LinkWithNoLabel covers the "else" branch of "if link.Label != ""
// in viewPageBody by viewing a task whose link has an empty label (shows URL text).
func TestViewTask_LinkWithNoLabel(t *testing.T) {
	taskID := createTask(t, "Task with unlabeled link")

	tEdit := test.CreateRequestTester(
		getRoutes(), http.MethodPost, "/todos/"+taskID+"/edit",
	)
	tEdit.SetContentType(test.FormContentType)
	tEdit.SetFollowRedirect(false)
	tEdit.SetData(
		dtos.SaveTaskDto{ //nolint:exhaustruct // only Title and LinkURLs needed
			Title:      "Task with unlabeled link",
			LinkURLs:   []string{"https://example.com/no-label"},
			LinkLabels: []string{""},
		},
	)
	require.Equal(t, http.StatusSeeOther, tEdit.Do(t).StatusCode)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/"+taskID, nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "https://example.com/no-label")
}

// TestListTasks_WithMdLinkTitle covers the "templates.HasMdLink(task.Title)" branch
// in taskRow by creating a task whose title contains a markdown link.
func TestListTasks_WithMdLinkTitle(t *testing.T) {
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title)
		VALUES ($1, '[click here](https://example.com) do the thing')`,
		userID,
	)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/todos/?w=private", nil)
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "click here")
}
