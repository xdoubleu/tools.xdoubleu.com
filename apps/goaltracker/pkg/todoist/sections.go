package todoist

import (
	"context"
	"fmt"
)

const SectionsEndpoint = "sections"

type Section struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	ProjectID    string `json:"project_id"`
	AddedAt      string `json:"added_at"`
	UpdatedAt    string `json:"updated_at"`
	ArchivedAt   string `json:"archived_at"`
	Name         string `json:"name"`
	SectionOrder int    `json:"section_order"`
	IsCollapsed  bool   `json:"is_collapsed"`
	IsArchived   bool   `json:"is_archived"`
	IsDeleted    bool   `json:"is_deleted"`
}

func (client client) GetAllSections(
	ctx context.Context,
	projectID string,
) ([]Section, error) {
	query := fmt.Sprintf("project_id=%s", projectID)

	var sections []Section
	err := getPaginatedResults(ctx, client, SectionsEndpoint, query, &sections)
	if err != nil {
		return nil, err
	}

	return sections, nil
}
