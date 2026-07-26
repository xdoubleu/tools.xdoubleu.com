package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
)

// MockArxivClient is a configurable in-memory arxiv.Client.
type MockArxivClient struct {
	// Papers maps arXiv id -> paper. Ids not present return arxiv.ErrNotFound.
	Papers map[string]*arxiv.Paper
	// Err, when set, is returned by every call (upstream failure).
	Err error
	// Calls records every requested id.
	Calls []string
	// HTML maps arXiv id -> HTML rendition bytes for GetHTML. Ids not
	// present in either HTML or HTMLErr return arxiv.ErrNotFound.
	HTML map[string][]byte
	// HTMLErr maps arXiv id -> an error GetHTML returns instead of
	// arxiv.ErrNotFound (simulates a non-404 upstream failure).
	HTMLErr map[string]error
}

// NewMockArxivClient returns an empty mock (every id is not found).
func NewMockArxivClient() *MockArxivClient {
	return &MockArxivClient{
		Papers:  map[string]*arxiv.Paper{},
		Err:     nil,
		Calls:   nil,
		HTML:    map[string][]byte{},
		HTMLErr: map[string]error{},
	}
}

func (m *MockArxivClient) GetByID(
	_ context.Context,
	id string,
) (*arxiv.Paper, error) {
	m.Calls = append(m.Calls, id)
	if m.Err != nil {
		return nil, m.Err
	}
	paper, ok := m.Papers[id]
	if !ok {
		return nil, arxiv.ErrNotFound
	}
	return paper, nil
}

func (m *MockArxivClient) GetHTML(
	_ context.Context,
	id string,
) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if err, ok := m.HTMLErr[id]; ok {
		return nil, err
	}
	if html, ok := m.HTML[id]; ok {
		return html, nil
	}
	return nil, arxiv.ErrNotFound
}
