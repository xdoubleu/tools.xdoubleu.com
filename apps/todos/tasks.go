package todos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/services"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func safeBackRedirect(back string) string {
	if back == "" {
		return todosRoot
	}

	normalized := strings.ReplaceAll(back, "\\", "/")
	target, err := url.Parse(normalized)
	if err != nil {
		return todosRoot
	}

	// Allow only local redirects (no host/scheme).
	if target.Hostname() != "" || target.IsAbs() {
		return todosRoot
	}

	// Require an absolute local path for predictable behavior.
	if !strings.HasPrefix(target.Path, "/") {
		return todosRoot
	}

	return target.String()
}

const todosRoot = "/todos/"

func isHXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

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

// ── List open tasks (optionally filtered by ?section=<uuid>) ─────────────────

// workspaceQuery returns the URL query segment that encodes the active workspace,
// e.g. "w=550e8400-…" or "w=private".
func workspaceQuery(wsID *uuid.UUID) string {
	if wsID == nil {
		return "w=private"
	}
	return "w=" + wsID.String()
}

func wsIDsEqual(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// applyWorkspaceParam reads ?w= from r, updates DB if it changed, and returns
// true + a redirect URL when the caller should redirect (missing ?w= on a full
// page load). The wsCtx is updated in-place when ?w= is present.
func (a *Todos) applyWorkspaceParam(
	r *http.Request,
	wsCtx *workspaceCtx,
	userID string,
) (string, bool) {
	rawW := r.URL.Query().Get("w")
	if rawW == "" {
		if isHXRequest(r) {
			return "", false // HTMX partial — use DB workspace, no redirect
		}
		target := "/todos/?" + workspaceQuery(wsCtx.Settings.ActiveWorkspaceID)
		if s := r.URL.Query().Get("section"); s != "" {
			target += "&section=" + s
		}
		return target, true
	}
	var newWsID *uuid.UUID
	if rawW != "private" {
		if id, parseErr := uuid.Parse(rawW); parseErr == nil {
			newWsID = &id
		}
	}
	if !wsIDsEqual(wsCtx.Settings.ActiveWorkspaceID, newWsID) {
		_ = a.services.Settings.SetActiveWorkspace(r.Context(), userID, newWsID)
		wsCtx.Settings.ActiveWorkspaceID = newWsID
		wsCtx.Settings.ActiveWorkspace = nil
		if newWsID != nil {
			for i := range wsCtx.Workspaces {
				if wsCtx.Workspaces[i].ID == *newWsID {
					wsCtx.Settings.ActiveWorkspace = &wsCtx.Workspaces[i]
					break
				}
			}
		}
	}
	return "", false
}

func (a *Todos) loadLabelColors(
	ctx context.Context,
	userID string,
	wsID *uuid.UUID,
) map[string]string {
	presets, err := a.services.Settings.GetLabelPresets(ctx, userID, wsID)
	if err != nil {
		return map[string]string{}
	}
	return presets.ColorMap()
}

func (a *Todos) listTasksHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)

	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}

	if target, redirect := a.applyWorkspaceParam(r, wsCtx, user.ID); redirect {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return nil
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

	if isHXRequest(r) {
		tpltools.RenderWithPanic(a.Tpl, w, "_task_list.html", map[string]any{
			"Tasks":          taskList,
			"CurrentSection": currentSection,
			"Sections":       sections,
			"LabelColors": a.loadLabelColors(
				r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
			),
		})
		return nil
	}

	return a.renderTaskList(
		w, r, user.ID, wsCtx, sections, taskList, currentSection, activeTab,
	)
}

func (a *Todos) renderTaskList(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	wsCtx *workspaceCtx,
	sections []models.Section,
	taskList []models.Task,
	currentSection *models.Section,
	activeTab string,
) error {
	wsID := wsCtx.Settings.ActiveWorkspaceID
	presets, err := a.services.Settings.GetLabelPresets(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	policies, err := a.services.Policies.List(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	patterns, err := a.services.Settings.GetURLPatterns(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	tabCounts, err := a.services.Tasks.CountOpenPerSection(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_list.html", map[string]any{
		"Tasks":          taskList,
		"Sections":       sections,
		"Presets":        presets,
		"LabelColors":    presets.ColorMap(),
		"Policies":       policies,
		"Patterns":       patterns,
		"ActiveTab":      activeTab,
		"CurrentSection": currentSection,
		"UserSettings":   wsCtx.Settings,
		"Workspaces":     wsCtx.Workspaces,
		"WorkspaceQuery": workspaceQuery(wsID),
		"TabCounts":      tabCounts,
	})
	return nil
}

// ── Quick-add (persistent input at top of list) ───────────────────────────────

func (a *Todos) resolveSection(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	rawSectionID string,
) (*uuid.UUID, *models.Section, error) {
	if rawSectionID == "" {
		return nil, nil, nil
	}
	sid, err := uuid.Parse(rawSectionID)
	if err != nil {
		return nil, nil, nil //nolint:nilerr // invalid UUID → treat as no section
	}
	sections, err := a.services.Sections.List(ctx, userID, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	for i := range sections {
		if sections[i].ID == sid {
			return &sid, &sections[i], nil
		}
	}
	return nil, nil, nil
}

func (a *Todos) quickAddHTMX(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	dto dtos.QuickAddDto,
	wsCtx *workspaceCtx,
) error {
	sectionID, currentSection, err := a.resolveSection(
		r.Context(), userID, wsCtx.Settings.ActiveWorkspaceID, dto.SectionID,
	)
	if err != nil {
		return err
	}
	taskList, err := a.services.Tasks.ListOpen(
		r.Context(), userID, sectionID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(
		r.Context(), userID, wsCtx.Settings.ActiveWorkspaceID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "_task_list.html", map[string]any{
		"Tasks":          taskList,
		"CurrentSection": currentSection,
		"Sections":       sections,
		"LabelColors": a.loadLabelColors(
			r.Context(), userID, wsCtx.Settings.ActiveWorkspaceID,
		),
	})
	return nil
}

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
	if _, err = a.services.Tasks.QuickAdd(
		r.Context(), user.ID, dto.Input, dto.Description,
		wsCtx.Settings.ActiveWorkspaceID, dto.SectionID,
	); err != nil {
		return err
	}

	if isHXRequest(r) {
		return a.quickAddHTMX(w, r, user.ID, dto, wsCtx)
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
		"LabelColors": a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
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
		"LabelColors": a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
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
		"LabelColors": a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
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
		"Task":       models.Task{}, //nolint:exhaustruct // empty task for new-task form
		"Action":     "/todos/new",
		"IsEdit":     false,
		"Presets":    presets,
		"Sections":   sections,
		"RecurInput": "",
	})
	return nil
}

// ── Create task ───────────────────────────────────────────────────────────────

func (a *Todos) createTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.SaveTaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if _, err = a.services.Tasks.Create(
		r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID, dto,
	); err != nil {
		return err
	}
	http.Redirect(w, r, todosRoot, http.StatusSeeOther)
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
		"Task":       task,
		"Action":     "/todos/" + id.String() + "/edit",
		"IsEdit":     true,
		"Presets":    presets,
		"Sections":   sections,
		"RecurInput": a.services.Tasks.FormatRecurRule(task.RecurRule, task.RecurDays),
	})
	return nil
}

func safeLocalRedirectTarget(rawBack string) string {
	if rawBack == "" {
		return todosRoot
	}

	normalized := strings.ReplaceAll(rawBack, "\\", "/")
	target, err := url.Parse(normalized)
	if err != nil {
		return todosRoot
	}

	if target.Hostname() != "" || target.Scheme != "" {
		return todosRoot
	}

	if !strings.HasPrefix(target.Path, "/") {
		return todosRoot
	}

	return target.String()
}

// ── Update task ───────────────────────────────────────────────────────────────

func (a *Todos) updateTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
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
	if err = a.services.Tasks.Update(
		r.Context(), id, user.ID, wsCtx.Settings.ActiveWorkspaceID, dto,
	); err != nil {
		return err
	}
	if r.Header.Get("X-Auto-Save") == "1" {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	rawBack := r.URL.Query().Get("back")
	var back string
	if rawBack == "" {
		back = "/todos/" + id.String() + "/edit"
	} else {
		back = safeLocalRedirectTarget(rawBack)
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
	if isHXRequest(r) {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	back := safeLocalRedirectTarget(r.URL.Query().Get("back"))
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
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	if err = a.services.Tasks.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}
	if isHXRequest(r) {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	back := safeBackRedirect(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Quick-update task (inline edit from list view) ────────────────────────────

func (a *Todos) quickUpdateHTMX(
	w http.ResponseWriter,
	r *http.Request,
	task *models.Task,
	userID string,
	wsID *uuid.UUID,
) error {
	sections, err := a.services.Sections.List(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	var currentSection *models.Section
	if sid, parseErr := uuid.Parse(r.URL.Query().Get("section")); parseErr == nil {
		for i := range sections {
			if sections[i].ID == sid {
				currentSection = &sections[i]
				break
			}
		}
	}
	tpltools.RenderWithPanic(a.Tpl, w, "_task_row", map[string]any{
		"Task":           task,
		"CurrentSection": currentSection,
		"LabelColors":    a.loadLabelColors(r.Context(), userID, wsID),
		"Sections":       sections,
	})
	return nil
}

func (a *Todos) quickUpdateHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.QuickAddDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	task, err := a.services.Tasks.QuickUpdate(
		r.Context(), id, user.ID,
		wsCtx.Settings.ActiveWorkspaceID,
		dto.Input, dto.Description,
	)
	if err != nil {
		return err
	}
	if isHXRequest(r) {
		return a.quickUpdateHTMX(
			w, r, task, user.ID, wsCtx.Settings.ActiveWorkspaceID,
		)
	}
	back := safeLocalRedirectTarget(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

// ── Move task to section ──────────────────────────────────────────────────────

func parseSectionID(raw string) *uuid.UUID {
	if raw == "" {
		return nil
	}
	if sid, err := uuid.Parse(raw); err == nil {
		return &sid
	}
	return nil
}

func (a *Todos) moveSectionHTMX(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	wsID *uuid.UUID,
) error {
	sections, err := a.services.Sections.List(r.Context(), userID, wsID)
	if err != nil {
		return err
	}
	currentSectionID := parseSectionID(r.URL.Query().Get("current"))
	var currentSection *models.Section
	if currentSectionID != nil {
		for i := range sections {
			if sections[i].ID == *currentSectionID {
				currentSection = &sections[i]
				break
			}
		}
	}
	taskList, err := a.services.Tasks.ListOpen(
		r.Context(), userID, currentSectionID, wsID,
	)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "_task_list.html", map[string]any{
		"Tasks":          taskList,
		"CurrentSection": currentSection,
		"Sections":       sections,
		"LabelColors":    a.loadLabelColors(r.Context(), userID, wsID),
	})
	return nil
}

func (a *Todos) moveSectionHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.MoveSectionDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	newSectionID := parseSectionID(dto.SectionID)
	if err = a.services.Tasks.MoveSection(
		r.Context(), id, user.ID, newSectionID,
	); err != nil {
		return err
	}
	if isHXRequest(r) {
		return a.moveSectionHTMX(
			w, r, user.ID, wsCtx.Settings.ActiveWorkspaceID,
		)
	}
	back := safeLocalRedirectTarget(r.URL.Query().Get("back"))
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
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddSubtaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	subtask, err := a.services.Tasks.AddSubtask(
		r.Context(), taskID, user.ID,
		wsCtx.Settings.ActiveWorkspaceID,
		dto.Input, dto.Description,
	)
	if err != nil {
		return err
	}
	if isHXRequest(r) {
		tpltools.RenderWithPanic(a.Tpl, w, "_subtask_item", map[string]any{
			"Subtask": subtask,
			"TaskID":  taskID,
		})
		return nil
	}
	back := safeBackRedirect(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) updateSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
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
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.UpdateSubtaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	_, err = a.services.Tasks.UpdateSubtask(
		r.Context(), sid, taskID, user.ID,
		wsCtx.Settings.ActiveWorkspaceID, dto,
	)
	if err != nil {
		return err
	}
	back := safeBackRedirect(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) reorderSubtasksHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	var dto dtos.ReorderDto
	if err = json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid JSON",
		}
	}
	ids := make([]uuid.UUID, 0, len(dto.IDs))
	for _, s := range dto.IDs {
		id, parseErr := uuid.Parse(s)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err = a.services.Tasks.ReorderSubtasks(
		r.Context(), taskID, user.ID, ids,
	); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
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
	if isHXRequest(r) {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	back := safeBackRedirect(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) toggleSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.ToggleSubtask)
}

func (a *Todos) deleteSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.DeleteSubtask)
}
