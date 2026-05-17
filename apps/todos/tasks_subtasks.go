package todos

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	iapp "tools.xdoubleu.com/internal/app"
)

func (a *Todos) addSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
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
	var dto dtos.AddSubtaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	var parentSubtaskID *uuid.UUID
	if dto.ParentSubtaskID != "" {
		sid, parseErr := uuid.Parse(dto.ParentSubtaskID)
		if parseErr != nil {
			return &iapp.HTTPError{
				Status:  http.StatusBadRequest,
				Message: "Invalid parent_subtask_id",
			}
		}
		parentSubtaskID = &sid
	}
	subtask, err := a.services.Tasks.AddSubtask(
		r.Context(), taskID, user.ID,
		wsCtx.Settings.ActiveWorkspaceID,
		dto.Input, dto.Description,
		parentSubtaskID,
	)
	if err != nil {
		return err
	}
	if isHXRequest(r) {
		lc := a.loadLabelColors(
			r.Context(),
			user.ID,
			wsCtx.Settings.ActiveWorkspaceID,
		)
		if dto.Source == subtaskSourceView {
			_ = SubtaskViewItemPartial(*subtask, taskID, 0, lc).Render(r.Context(), w)
		} else {
			_ = SubtaskItemPartial(*subtask, taskID, 0, lc).Render(r.Context(), w)
		}
		return nil
	}
	back := safeBackRedirect(r.URL.Query().Get("back"))
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}

func (a *Todos) addNestedSubtaskHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	parentSubtaskID, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Parent subtask not found",
		}
	}
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddSubtaskDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	subtask, err := a.services.Tasks.AddSubtask(
		r.Context(), taskID, user.ID,
		wsCtx.Settings.ActiveWorkspaceID,
		dto.Input, dto.Description,
		&parentSubtaskID,
	)
	if err != nil {
		return err
	}
	if isHXRequest(r) {
		parentDepth, depthErr := a.repos.Tasks.GetSubtaskDepth(
			r.Context(), taskID, parentSubtaskID,
		)
		if depthErr != nil {
			return depthErr
		}
		lc := a.loadLabelColors(r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID)
		if dto.Source == subtaskSourceView {
			_ = SubtaskViewItemPartial(
				*subtask,
				taskID,
				parentDepth+1,
				lc,
			).Render(r.Context(), w)
		} else {
			_ = SubtaskItemPartial(*subtask, taskID, parentDepth+1, lc).Render(r.Context(), w)
		}
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
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		return &iapp.HTTPError{
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
		return &iapp.HTTPError{
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
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	var dto dtos.ReorderSubtasksDto
	if err = json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return &iapp.HTTPError{
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
	var parentSubtaskID *uuid.UUID
	if dto.ParentSubtaskID != "" {
		sid, parseErr := uuid.Parse(dto.ParentSubtaskID)
		if parseErr != nil {
			return &iapp.HTTPError{
				Status:  http.StatusBadRequest,
				Message: "Invalid parent_subtask_id",
			}
		}
		parentSubtaskID = &sid
	}
	if err = a.services.Tasks.ReorderSubtasks(
		r.Context(), taskID, user.ID, ids, parentSubtaskID,
	); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *Todos) renderSubtaskListAfterAction(
	w http.ResponseWriter,
	r *http.Request,
	taskID uuid.UUID,
	userID string,
) error {
	task, err := a.services.Tasks.Get(r.Context(), taskID, userID)
	if err != nil {
		return err
	}
	source := r.FormValue("source") //nolint:gosec // form already parsed
	lc := a.loadLabelColors(r.Context(), userID, task.WorkspaceID)
	if source == subtaskSourceView {
		_ = SubtaskViewListPartial(task.Subtasks, task.ID, lc).Render(r.Context(), w)
	} else {
		_ = SubtaskListPartial(task.Subtasks, task.ID, lc).Render(r.Context(), w)
	}
	return nil
}

func (a *Todos) handleSubtaskAction(
	w http.ResponseWriter,
	r *http.Request,
	action func(context.Context, uuid.UUID, uuid.UUID, string) error,
	renderAfter bool,
) error {
	user := currentUser(r)
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Task not found",
		}
	}
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Subtask not found",
		}
	}
	if err = action(r.Context(), sid, taskID, user.ID); err != nil {
		return err
	}
	if !isHXRequest(r) {
		back := safeBackRedirect(r.URL.Query().Get("back"))
		http.Redirect(w, r, back, http.StatusSeeOther)
		return nil
	}
	if !renderAfter {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	return a.renderSubtaskListAfterAction(w, r, taskID, user.ID)
}

func (a *Todos) toggleSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.ToggleSubtask, true)
}

func (a *Todos) deleteSubtaskHandler(w http.ResponseWriter, r *http.Request) error {
	return a.handleSubtaskAction(w, r, a.services.Tasks.DeleteSubtask, true)
}
