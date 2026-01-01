package services

import (
	"context"

	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type TodoistService struct {
	client    todoist.Client
	projectID string
}

func (service *TodoistService) GetSections(
	ctx context.Context,
) ([]todoist.Section, error) {
	sections, err := service.client.GetAllSections(ctx, service.projectID)
	if err != nil {
		return nil, err
	}

	return sections, nil
}

func (service *TodoistService) GetTasks(ctx context.Context) ([]todoist.Task, error) {
	tasks, err := service.client.GetActiveTasks(ctx, service.projectID)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(tasks); i++ {
		if tasks[i].ParentID == nil {
			continue
		}

		tasks = append(tasks[:i], tasks[i+1:]...)
		i--
	}

	return tasks, nil
}

func (service *TodoistService) GetTaskByID(
	ctx context.Context,
	id string,
) (*todoist.Task, error) {
	return service.client.GetActiveTask(ctx, id)
}

func (service *TodoistService) CompleteTask(
	ctx context.Context,
	id string,
) error {
	return service.client.CloseTask(ctx, id)
}
