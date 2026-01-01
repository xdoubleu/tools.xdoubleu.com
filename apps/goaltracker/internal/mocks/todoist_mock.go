//nolint:exhaustruct,revive //ignore
package mocks

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type MockTodoistClient struct {
}

func NewMockTodoistClient() todoist.Client {
	return MockTodoistClient{}
}

func (client MockTodoistClient) GetActiveTask(
	ctx context.Context,
	id string,
) (*todoist.Task, error) {
	return &todoist.Task{}, nil
}

func (client MockTodoistClient) GetActiveTasks(
	ctx context.Context,
	projectID string,
) ([]todoist.Task, error) {
	return []todoist.Task{
		{
			ID: "123",
			Due: &todoist.Due{
				String: "every year",
				Date: todoist.Date{
					Time: time.Date(time.Now().Year(), 1, 1, 10, 0, 0, 0, time.UTC),
				},
				IsRecurring: true,
			},
		},
		{
			ID: "456",
			Due: &todoist.Due{
				String: "every year",
				Date: todoist.Date{
					Time: time.Date(time.Now().Year(), 1, 1, 10, 0, 0, 0, time.UTC),
				},
				IsRecurring: true,
			},
		},
	}, nil
}

func (client MockTodoistClient) GetAllProjects(
	ctx context.Context,
) ([]todoist.Project, error) {
	return []todoist.Project{}, nil
}

func (client MockTodoistClient) GetAllSections(
	ctx context.Context,
	projectID string,
) ([]todoist.Section, error) {
	return []todoist.Section{
		{
			ID:    "1",
			Name:  "State1",
			Order: 1,
		},
	}, nil
}

func (client MockTodoistClient) UpdateTask(
	ctx context.Context,
	taskID string,
	updateTaskDto todoist.UpdateTaskDto,
) (*todoist.Task, error) {
	return &todoist.Task{}, nil
}

func (client MockTodoistClient) CloseTask(ctx context.Context, taskID string) error {
	return nil
}
