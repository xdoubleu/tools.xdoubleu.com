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
}

// NewMockArxivClient returns an empty mock (every id is not found).
func NewMockArxivClient() *MockArxivClient {
	return &MockArxivClient{Papers: map[string]*arxiv.Paper{}, Err: nil, Calls: nil}
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
