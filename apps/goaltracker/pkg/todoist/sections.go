package todoist

import (
	"context"
	"fmt"
	"net/http"
)

const SectionsEndpoint = "sections"

type Section struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Order     int    `json:"order"`
	Name      string `json:"name"`
}

func (client client) GetAllSections(
	ctx context.Context,
	projectID string,
) ([]Section, error) {
	query := fmt.Sprintf("project_id=%s", projectID)

	var sections []Section
	err := client.sendRequest(
		ctx,
		http.MethodGet,
		SectionsEndpoint,
		query,
		nil,
		&sections,
	)
	if err != nil {
		return nil, err
	}

	return sections, nil
}
