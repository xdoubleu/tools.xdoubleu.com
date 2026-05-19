package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

type MockHardcoverClient struct{}

func (m MockHardcoverClient) Search(
	_ context.Context,
	_ string,
) ([]hardcover.ExternalBook, error) {
	isbn := "9780140447934"
	cover := "https://example.com/cover.jpg"
	desc := "A test book."
	return []hardcover.ExternalBook{
		{ //nolint:exhaustruct //ISBN10 not needed for mock
			Provider:    "hardcover",
			ProviderID:  "1",
			Title:       "The Odyssey",
			Authors:     []string{"Homer"},
			ISBN13:      &isbn,
			CoverURL:    &cover,
			Description: &desc,
		},
	}, nil
}

func (m MockHardcoverClient) GetByID(
	_ context.Context,
	_ string,
) (*hardcover.ExternalBook, error) {
	return nil, nil //nolint:nilnil //interface contract allows nil, nil for not-found
}

func NewMockHardcoverClient() hardcover.Client {
	return MockHardcoverClient{}
}
