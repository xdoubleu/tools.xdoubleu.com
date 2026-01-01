package todoist

import (
	"context"
	"net/http"
)

const ProjectsEndpoint = "projects"

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Color          string `json:"color"`
	ParentID       string `json:"parent_id"`
	Order          int    `json:"order"`
	CommentCount   int    `json:"comment_count"`
	IsShared       bool   `json:"is_shared"`
	IsFavorite     bool   `json:"is_favorite"`
	IsInboxProject bool   `json:"is_inbox_project"`
	IsTeamInbox    bool   `json:"is_team_inbox"`
	ViewStyle      string `json:"view_style"`
	URL            string `json:"url"`
}

func (client client) GetAllProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	err := client.sendRequest(ctx, http.MethodGet, ProjectsEndpoint, "", nil, &projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}
