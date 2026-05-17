package todos

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
)

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
	_ = FormPage(FormPageData{
		Task:       models.Task{}, //nolint:exhaustruct // empty task for new-task form
		Action:     "/todos/new",
		IsEdit:     false,
		Presets:    *presets,
		Sections:   sections,
		RecurInput: "",
	}).Render(r.Context(), w)
	return nil
}

func (a *Todos) createTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.SaveTaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
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

func (a *Todos) viewTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	task, err := a.services.Tasks.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}

	_ = ViewPage(ViewPageData{
		Task:     *task,
		DescHTML: task.Description,
		LabelColors: a.loadLabelColors(
			r.Context(),
			user.ID,
			task.WorkspaceID,
		),
	}).Render(r.Context(), w)
	return nil
}

func (a *Todos) editTaskFormHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
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
	_ = FormPage(FormPageData{
		Task:       *task,
		Action:     "/todos/" + id.String() + "/edit",
		IsEdit:     true,
		Presets:    *presets,
		Sections:   sections,
		RecurInput: a.services.Tasks.FormatRecurRule(task.RecurRule, task.RecurDays),
	}).Render(r.Context(), w)
	return nil
}

func (a *Todos) updateTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	var dto dtos.SaveTaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
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

func (a *Todos) completeTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
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

func (a *Todos) reopenTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
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

func (a *Todos) deleteTaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
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
	_ = TaskRowPartial(
		*task,
		currentSection,
		a.loadLabelColors(r.Context(), userID, wsID),
		sections,
	).Render(r.Context(), w)
	return nil
}

func (a *Todos) quickUpdateHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
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
		return &iapp.HTTPError{
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

func (a *Todos) reorderHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.ReorderDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return &iapp.HTTPError{
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
