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
}

func NewMockTodoistClient(taskID string) *MockTodoistClient {
	return &MockTodoistClient{TaskID: taskID} //nolint:exhaustruct // zero values fine
}

func (m *MockTodoistClient) CreateTask(
	_ context.Context, content, dueString string,
) (string, error) {
	m.LastContent = content
	m.LastDueString = dueString
	if m.Err != nil {
		return "", m.Err
	}
	return m.TaskID, nil
}
