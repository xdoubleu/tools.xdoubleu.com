// Package mocks holds hand-written test doubles for learningpaths'
// external dependencies, so the test suite never makes a real network call
// (issue #1475's Todoist integration in particular).
package mocks

import "context"

// MockTodoistClient returns a canned task id from CreateTask, recording the
// last call's arguments for assertions.
type MockTodoistClient struct {
	TaskID string
	Err    error

	LastContent   string
	LastDueString string
	LastProjectID string
	LastDeletedID string

	// ProjectID is returned by CreateProject; ProjectMissing makes
	// ProjectExists report false.
	ProjectID            string
	ProjectMissing       bool
	CreatedProjects      int
	LastDeletedProjectID string
}

func NewMockTodoistClient(taskID string) *MockTodoistClient {
	return &MockTodoistClient{TaskID: taskID} //nolint:exhaustruct // zero values fine
}

func (m *MockTodoistClient) CreateTask(
	_ context.Context, content, dueString, projectID string,
) (string, error) {
	m.LastContent = content
	m.LastDueString = dueString
	m.LastProjectID = projectID
	if m.Err != nil {
		return "", m.Err
	}
	return m.TaskID, nil
}

func (m *MockTodoistClient) DeleteTask(_ context.Context, taskID string) error {
	m.LastDeletedID = taskID
	if m.Err != nil {
		return m.Err
	}
	return nil
}

func (m *MockTodoistClient) CreateProject(_ context.Context, _ string) (string, error) {
	m.CreatedProjects++
	return m.ProjectID, m.Err
}

func (m *MockTodoistClient) ProjectExists(_ context.Context, _ string) (bool, error) {
	return !m.ProjectMissing, m.Err
}

func (m *MockTodoistClient) DeleteProject(_ context.Context, projectID string) error {
	m.LastDeletedProjectID = projectID
	return m.Err
}
