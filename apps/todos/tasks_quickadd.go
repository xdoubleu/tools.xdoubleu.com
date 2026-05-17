package todos

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
)

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
	_ = TaskListPartial(taskListData{
		Tasks:          taskList,
		CurrentSection: currentSection,
		Sections:       sections,
		LabelColors: a.loadLabelColors(
			r.Context(), userID, wsCtx.Settings.ActiveWorkspaceID,
		),
	}).Render(r.Context(), w)
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
	_ = TaskListPartial(taskListData{
		Tasks:          taskList,
		CurrentSection: currentSection,
		Sections:       sections,
		LabelColors:    a.loadLabelColors(r.Context(), userID, wsID),
	}).Render(r.Context(), w)
	return nil
}

func (a *Todos) moveSectionHandler(w http.ResponseWriter, r *http.Request) error {
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
	var dto dtos.MoveSectionDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
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
