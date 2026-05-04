package todos

import (
	"net/http"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/services"
)

func (a *Todos) settingsHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	wsID := wsCtx.Settings.ActiveWorkspaceID
	presets, err := a.services.Settings.GetLabelPresets(r.Context(), user.ID, wsID)
	if err != nil {
		return err
	}
	patterns, err := a.services.Settings.GetURLPatterns(r.Context(), user.ID, wsID)
	if err != nil {
		return err
	}
	archive, err := a.services.Settings.GetArchiveSettings(r.Context(), user.ID)
	if err != nil {
		return err
	}
	sections, err := a.services.Sections.List(r.Context(), user.ID, wsID)
	if err != nil {
		return err
	}
	policies, err := a.services.Policies.List(r.Context(), user.ID, wsID)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "todos_settings.html", map[string]any{
		"Presets":      presets,
		"Patterns":     patterns,
		"Archive":      archive,
		"Sections":     sections,
		"Policies":     policies,
		"Workspaces":   wsCtx.Workspaces,
		"UserSettings": wsCtx.Settings,
	})
	return nil
}

func (a *Todos) updateArchiveHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.UpdateArchiveDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if err := a.services.Settings.UpdateArchiveSettings(
		r.Context(), user.ID, dto.ArchiveAfterHours,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) addLabelHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddLabelPresetDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if err = a.services.Settings.AddLabelPreset(
		r.Context(), user.ID, dto, wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) removeLabelHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	category := r.PathValue("category")
	value := r.PathValue("value")
	if err = a.services.Settings.RemoveLabelPreset(
		r.Context(), user.ID, category, value, wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) addURLPatternHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddURLPatternDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if err = a.services.Settings.AddURLPattern(
		r.Context(), user.ID, dto, wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) removeURLPatternHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid pattern ID",
		}
	}
	if err = a.services.Settings.RemoveURLPattern(r.Context(), id, user.ID); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) addSectionHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddSectionDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if _, err = a.services.Sections.Create(
		r.Context(), user.ID, dto, wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) removeSectionHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid section ID",
		}
	}
	if err = a.services.Sections.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) addPolicyHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	wsCtx, err := a.loadWorkspaceCtx(r.Context(), user.ID)
	if err != nil {
		return err
	}
	var dto dtos.AddPolicyDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if _, err = a.services.Policies.Create(
		r.Context(), user.ID, dto.Text, dto.ReappearAfterHours,
		wsCtx.Settings.ActiveWorkspaceID,
	); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) removePolicyHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid policy ID",
		}
	}
	if err = a.services.Policies.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) addWorkspaceHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.AddWorkspaceDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	if _, err := a.services.Workspaces.Create(r.Context(), user.ID, dto.Name); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) deleteWorkspaceHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid workspace ID",
		}
	}
	if err = a.services.Workspaces.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}
	http.Redirect(w, r, "/todos/settings", http.StatusSeeOther)
	return nil
}

func (a *Todos) setModeHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	var dto dtos.SetModeDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}
	var workspaceID *uuid.UUID
	if dto.WorkspaceID != "" {
		id, err := uuid.Parse(dto.WorkspaceID)
		if err != nil {
			return &services.HTTPError{
				Status:  http.StatusBadRequest,
				Message: "Invalid workspace ID",
			}
		}
		workspaceID = &id
	}
	if err := a.services.Settings.SetActiveWorkspace(
		r.Context(), user.ID, workspaceID,
	); err != nil {
		return err
	}
	back := dto.Back
	if back == "" {
		back = todosRoot
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
	return nil
}
