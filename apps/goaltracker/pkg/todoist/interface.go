package todoist

import (
	"context"
)

type Client interface {
	GetAllProjects(ctx context.Context) ([]Project, error)
	GetAllSections(ctx context.Context, projectID string) ([]Section, error)
	GetActiveTasks(ctx context.Context, projectID string) ([]Task, error)
	GetActiveTask(ctx context.Context, taskID string) (*Task, error)
	UpdateTask(
		ctx context.Context,
		taskID string,
		updateTaskDto UpdateTaskDto,
	) (*Task, error)
	CloseTask(ctx context.Context, taskID string) error
}
