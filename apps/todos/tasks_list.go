package todos

import (
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
)

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
	activeTab := tabOpen

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
			return &iapp.HTTPError{
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
			return &iapp.HTTPError{
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
		_ = TaskListPartial(taskListData{
			Tasks:          taskList,
			CurrentSection: currentSection,
			Sections:       sections,
			LabelColors: a.loadLabelColors(
				r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
			),
		}).Render(r.Context(), w)
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
	_ = ListPage(ListPageData{
		Tasks:          taskList,
		Sections:       sections,
		Presets:        *presets,
		LabelColors:    presets.ColorMap(),
		Policies:       policies,
		Patterns:       patterns,
		ActiveTab:      activeTab,
		CurrentSection: currentSection,
		UserSettings:   wsCtx.Settings,
		Workspaces:     wsCtx.Workspaces,
		WorkspaceQuery: workspaceQuery(wsID),
		TabCounts:      tabCounts,
	}).Render(r.Context(), w)
	return nil
}

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
	_ = SearchPage(SearchPageData{
		Query:        query,
		Results:      results,
		Sections:     sections,
		UserSettings: wsCtx.Settings,
		Workspaces:   wsCtx.Workspaces,
		LabelColors: a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
	}).Render(r.Context(), w)
	return nil
}

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
	_ = DonePage(DonePageData{
		Tasks:             taskList,
		ArchiveAfterHours: archiveSettings.ArchiveAfterHours,
		Sections:          sections,
		UserSettings:      wsCtx.Settings,
		Workspaces:        wsCtx.Workspaces,
		LabelColors: a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
	}).Render(r.Context(), w)
	return nil
}

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
	_ = ArchivePage(ArchivePageData{
		Tasks:        taskList,
		Query:        query,
		Sections:     sections,
		UserSettings: wsCtx.Settings,
		Workspaces:   wsCtx.Workspaces,
		LabelColors: a.loadLabelColors(
			r.Context(), user.ID, wsCtx.Settings.ActiveWorkspaceID,
		),
	}).Render(r.Context(), w)
	return nil
}
