package services

import (
	"context"

	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type TodoistService struct {
	clientFactory func(apiKey string) todoist.Client
}

func (service *TodoistService) GetSections(
	ctx context.Context,
	creds repositories.UserIntegrations,
) ([]todoist.Section, error) {
	sections, err := service.clientFactory(creds.TodoistAPIKey).GetAllSections(
		ctx,
		creds.TodoistProjectID,
	)
	if err != nil {
		return nil, err
	}
	return sections, nil
}

func (service *TodoistService) GetTasks(
	ctx context.Context,
	creds repositories.UserIntegrations,
) ([]todoist.Task, error) {
	tasks, err := service.clientFactory(creds.TodoistAPIKey).GetActiveTasks(
		ctx,
		creds.TodoistProjectID,
	)
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
	creds repositories.UserIntegrations,
) (*todoist.Task, error) {
	return service.clientFactory(creds.TodoistAPIKey).GetActiveTask(ctx, id)
}

func (service *TodoistService) CompleteTask(
	ctx context.Context,
	id string,
	creds repositories.UserIntegrations,
) error {
	return service.clientFactory(creds.TodoistAPIKey).CloseTask(ctx, id)
}
