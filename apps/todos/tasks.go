package todos

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"github.com/yuin/goldmark"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/services"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

//nolint:gochecknoglobals // stateless renderers, safe to share
var (
	md        = goldmark.New()
	sanitizer = bluemonday.UGCPolicy()
func safeBackRedirect(back string, fallback string) string {
	if back == "" {
		return fallback
	}

	normalized := strings.ReplaceAll(back, "\\", "/")
	target, err := url.Parse(normalized)
	if err != nil {
		return fallback
	}

	if target.Hostname() != "" || !strings.HasPrefix(target.Path, "/") {
		return fallback
	}

	return target.String()
}

)

const todosRoot = "/todos/"

type workspaceCtx struct {
	Settings   *models.UserSettings
	Workspaces []models.Workspace
}

func (a *Todos) loadWorkspaceCtx(
	ctx context.Context,
	userID string,
) (*workspaceCtx, error) {
	settings, err := a.services.Settings.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	workspaces, err := a.services.Workspaces.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &workspaceCtx{Settings: settings, Workspaces: workspaces}, nil
}

type SearchResults struct {
	Open     []models.Task
	Done     []models.Task
	Archived []models.Task
}

func currentUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

func renderMarkdown(src string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	safe := sanitizer.SanitizeBytes(buf.Bytes())
	return template.HTML(safe), nil //nolint:gosec // sanitised by bluemonday
}

// ── List open tasks (optionally filtered by ?section=<uuid>) ─────────────────

func (a *Todos) listTasksHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)

	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}

	var sectionID *uuid.UUID
	var currentSection *models.Section
	activeTab := "open"

	sections, err := a.services.Sections.List(
		r.Context(),
		user.ID,
		wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}

	if raw := r.URL.Query().Get("section"); raw != "" {
		sid, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return &services.HTTPError{
				Status:  http.StatusNotFound,
				Message: "Section not found",
			}
		}
		for i := range sections {
			if sections[i].ID == sid {
				currentSection = &sections[i]
				break
			}
		}
		if currentSection == nil {
			return &services.HTTPError{
				Status:  http.StatusNotFound,
				Message: "Section not found",
			}
		}
		sectionID = &sid
		activeTab = sid.String()
	}

	taskList, err := a.services.Tasks.ListOpen(
		r.Context(), user.ID, sectionID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	presets, err := a.services.Settings.GetLabelPresets(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	policies, err := a.services.Policies.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_list.html", map[string]any{
		"Tasks":          taskList,
		"Sections":       sections,
		"Presets":        presets,
		"Policies":       policies,
		"ActiveTab":      activeTab,
		"CurrentSection": currentSection,
		"UserSettings":   wsCtx.Settings,
		"Workspaces":     wsCtx.Workspaces,
	})
	return nil
}

// ── Quick-add (persistent input at top of list) ───────────────────────────────

func (a *Todos) quickAddHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.QuickAddDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		http.Redirect(w, r, todosRoot, http.StatusSeeOther)
		return nil //nolint:nilerr // bad form data → silently redirect
	}
	if dto.Input == "" {
		http.Redirect(w, r, todosRoot, http.StatusSeeOther)
		return nil
	}
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	if err = a.services.Tasks.QuickAdd(
		r.Context(), user.ID, dto.Input, wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = todosRoot
	} else {
		back = strings.ReplaceAll(back, "\\", "/")
		target, parseErr := url.Parse(back)
		if parseErr != nil || target.Hostname() != "" {
			back = todosRoot
		}
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Reorder tasks (drag-drop) ─────────────────────────────────────────────────

func (a *Todos) reorderHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.ReorderDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid JSON",
		}
	}
	ids := make([]uuid.UUID, 0, len(dto.IDs))
	for _, s := range dto.IDs {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err := a.services.Tasks.Reorder(r.Context(), user.ID, ids); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ── Search across all statuses ────────────────────────────────────────────────

func (a *Todos) searchHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	query := r.URL.Query().Get("q")
	sections, err := a.services.Sections.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	var results SearchResults
	if query != "" {
		all, searchErr := a.services.Tasks.SearchAll(
			r.Context(), user.ID, query, wsCtx.Settings.ActiveWorkspaceID,
		)
		if searchErr != nil {
			return searchErr
		}
		for _, t := range all {
			switch t.Status {
			case models.StatusOpen:
				results.Open = append(results.Open, t)
			case models.StatusDone:
				results.Done = append(results.Done, t)
			default:
				results.Archived = append(results.Archived, t)
			}
		}
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_search.html", map[string]any{
		"Query":        query,
		"Results":      results,
		"Sections":     sections,
		"ActiveTab":    "search",
		"UserSettings": wsCtx.Settings,
		"Workspaces":   wsCtx.Workspaces,
	})
	return nil
}

// ── Done tasks ────────────────────────────────────────────────────────────────

func (a *Todos) listDoneHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	taskList, err := a.services.Tasks.List(
		r.Context(), user.ID, models.StatusDone, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	archiveSettings, err := a.services.Settings.GetArchiveSettings(r.Context(), user.ID)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_list_done.html", map[string]any{
		"Tasks":             taskList,
		"ArchiveAfterHours": archiveSettings.ArchiveAfterHours,
		"Sections":          sections,
		"ActiveTab":         "done",
		"UserSettings":      wsCtx.Settings,
		"Workspaces":        wsCtx.Workspaces,
	})
	return nil
}

// ── Archived tasks ────────────────────────────────────────────────────────────

func (a *Todos) listArchiveHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	query := r.URL.Query().Get("q")
	taskList, err := a.services.Tasks.Search(
		r.Context(), user.ID, query, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_list_archive.html", map[string]any{
		"Tasks":        taskList,
		"Query":        query,
		"Sections":     sections,
		"ActiveTab":    "archive",
		"UserSettings": wsCtx.Settings,
		"Workspaces":   wsCtx.Workspaces,
	})
	return nil
}

// ── New task form ─────────────────────────────────────────────────────────────

func (a *Todos) newTaskFormHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	presets, err := a.services.Settings.GetLabelPresets(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_form.html", map[string]any{
		"Task":     models.Task{}, //nolint:exhaustruct // empty task for new-task form
		"Action":   "/todos/new",
		"IsEdit":   false,
		"Presets":  presets,
		"Sections": sections,
	})
	return nil
}

// ── Create task ───────────────────────────────────────────────────────────────

func (a *Todos) createTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.SaveTaskDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if _, err := a.services.Tasks.Create(r.Context(), user.ID, dto); err != nil {
		return err
	}
	http.Redirect(w, r, todosRoot, http.StatusSeeOther)
	return nil
}

// ── View task ─────────────────────────────────────────────────────────────────

func (a *Todos) viewTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	task, err := a.services.Tasks.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}
	descHTML, err := renderMarkdown(task.Description)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_view.html", map[string]any{
		"Task":     task,
		"DescHTML": descHTML,
	})
	return nil
}

// ── Edit task form ────────────────────────────────────────────────────────────

func (a *Todos) editTaskFormHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	task, err := a.services.Tasks.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	presets, err := a.services.Settings.GetLabelPresets(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_form.html", map[string]any{
		"Task":     task,
		"Action":   "/todos/" + id.String() + "/edit",
		"IsEdit":   true,
		"Presets":  presets,
		"Sections": sections,
	})
	return nil
}

// ── Update task ───────────────────────────────────────────────────────────────

func (a *Todos) updateTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	var dto dtos.SaveTaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if err = a.services.Tasks.Update(r.Context(), id, user.ID, dto); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = "/todos/" + id.String()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Complete task ─────────────────────────────────────────────────────────────

func (a *Todos) completeTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	if err = a.services.Tasks.Complete(r.Context(), id, user.ID); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = todosRoot
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Reopen task ───────────────────────────────────────────────────────────────

func (a *Todos) reopenTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	if err = a.services.Tasks.Reopen(r.Context(), id, user.ID); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/done", http.StatusSeeOther)
	return nil
}

// ── Delete task ───────────────────────────────────────────────────────────────

func (a *Todos) deleteTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
	back := safeBackRedirect(r.URL.Query().Get("back"), todosRoot)
	}
	if err = a.services.Tasks.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = todosRoot
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Subtask handlers ──────────────────────────────────────────────────────────

func (a *Todos) addSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	var dto dtos.AddSubtaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
	back := safeBackRedirect(r.URL.Query().Get("back"), "/todos/"+taskID.String())
		r.Context(), taskID, user.ID, dto.Title,
	); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = "/todos/" + taskID.String()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) handleSubtaskAction(
	w http.ResponseWriter,
	r *http.Request,
	action func(context.Context, uuid.UUID, uuid.UUID, string) error,
) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Subtask not found",
		}
	}
	if err = action(r.Context(), sid, taskID, user.ID); err != nil {
		return err
	}
	back := r.URL.Query().Get("back")
	if back == "" {
		back = "/todos/" + taskID.String()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) toggleSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.ToggleSubtask)
}

func (a *Todos) deleteSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.DeleteSubtask)
}
