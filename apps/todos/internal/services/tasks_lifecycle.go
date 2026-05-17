package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type TaskService struct {
	tasks    *repositories.TasksRepository
	settings *repositories.SettingsRepository
	sections *repositories.SectionsRepository
}

func (s *TaskService) ListOpen(
	ctx context.Context,
	userID string,
	sectionID *uuid.UUID,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	tasks, err := s.tasks.ListOpen(ctx, userID, sectionID, workspaceID)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachSubtasks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachLinks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	return s.enrichWithShortcuts(ctx, userID, workspaceID, tasks), nil
}

func (s *TaskService) CountOpenPerSection(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) (map[string]int, error) {
	return s.tasks.CountOpenPerSection(ctx, userID, workspaceID)
}

func (s *TaskService) List(
	ctx context.Context,
	userID string,
	status string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	tasks, err := s.tasks.ListByStatus(ctx, userID, status, workspaceID)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachSubtasks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	tasks, err = s.attachLinks(ctx, tasks)
	if err != nil {
		return nil, err
	}
	return s.enrichWithShortcuts(ctx, userID, workspaceID, tasks), nil
}

func (s *TaskService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Task, error) {
	task, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	// Build subtask tree and recalculate counts
	task.Subtasks = buildSubtaskTree(task.Subtasks)
	task.SubtaskTotal = countSubtasksRecursive(task.Subtasks)
	task.SubtaskDone = countDoneSubtasksRecursive(task.Subtasks)

	if len(task.Links) > 0 {
		tasks := s.enrichWithShortcuts(
			ctx, userID, task.WorkspaceID, []models.Task{*task},
		)
		*task = tasks[0]
	}
	return task, nil
}

func (s *TaskService) Create(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	dto dtos.SaveTaskDto,
) (*models.Task, error) {
	dueDate, deadline, recurDays, recurRule, err := parseScheduleDTO(dto, time.Now())
	if err != nil {
		return nil, err
	}

	labels := s.normalizeAndAddLabels(
		ctx,
		userID,
		workspaceID,
		parseLabelsInput(dto.Label),
	)

	//nolint:exhaustruct // ID, Status, timestamps set by DB
	t := models.Task{
		OwnerUserID: userID,
		Title:       dto.Title,
		Description: dto.Description,
		Labels:      labels,
		Priority:    dto.Priority,
		RecurDays:   recurDays,
		RecurRule:   recurRule,
		DueDate:     dueDate,
		Deadline:    deadline,
		SectionID:   parseSectionID(dto.SectionID),
	}
	created, err := s.tasks.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	links := dtoToLinks(dto, created.ID)
	if err = s.tasks.ReplaceLinks(ctx, created.ID, links); err != nil {
		return nil, err
	}
	created.Links = links
	return created, nil
}

func (s *TaskService) Update(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	workspaceID *uuid.UUID,
	dto dtos.SaveTaskDto,
) error {
	dueDate, deadline, recurDays, recurRule, err := parseScheduleDTO(dto, time.Now())
	if err != nil {
		return err
	}

	labels := s.normalizeAndAddLabels(
		ctx,
		userID,
		workspaceID,
		parseLabelsInput(dto.Label),
	)

	existing, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	existing.Title = dto.Title
	existing.Description = dto.Description
	existing.Labels = labels
	existing.Priority = dto.Priority
	existing.RecurDays = recurDays
	existing.RecurRule = recurRule
	existing.DueDate = dueDate
	existing.Deadline = deadline
	existing.SectionID = parseSectionID(dto.SectionID)
	if err = s.tasks.Update(ctx, *existing); err != nil {
		return err
	}
	return s.tasks.ReplaceLinks(ctx, id, dtoToLinks(dto, id))
}

func (s *TaskService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.tasks.Delete(ctx, id, userID)
}

func (s *TaskService) MoveSection(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	sectionID *uuid.UUID,
) error {
	return s.tasks.MoveSection(ctx, id, userID, sectionID)
}

func (s *TaskService) Complete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	task, err := s.tasks.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err = s.tasks.SetStatus(
		ctx, id, userID, models.StatusDone, &now, nil,
	); err != nil {
		return err
	}
	if task.RecurDays <= 0 && task.RecurRule == "" {
		return nil
	}
	due, recurDays := nextRecurringDue(now, task.RecurRule, task.RecurDays)
	if due == nil {
		return nil
	}
	//nolint:exhaustruct // ID/Status/timestamps set by DB
	newTask := models.Task{
		OwnerUserID: task.OwnerUserID,
		Title:       task.Title,
		Description: task.Description,
		Labels:      task.Labels,
		Priority:    task.Priority,
		RecurDays:   recurDays,
		RecurRule:   task.RecurRule,
		DueDate:     due,
		SectionID:   task.SectionID,
		WorkspaceID: task.WorkspaceID,
	}
	created, err := s.tasks.Create(ctx, newTask)
	if err != nil {
		return err
	}
	if len(task.Links) == 0 {
		return nil
	}
	links := make([]models.TaskLink, len(task.Links))
	for i, l := range task.Links {
		//nolint:exhaustruct // ID set by DB
		links[i] = models.TaskLink{
			TaskID:    created.ID,
			URL:       l.URL,
			Label:     l.Label,
			SortOrder: l.SortOrder,
		}
	}
	return s.tasks.ReplaceLinks(ctx, created.ID, links)
}

func (s *TaskService) Reopen(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.tasks.SetStatus(ctx, id, userID, models.StatusOpen, nil, nil)
}

func (s *TaskService) Reorder(
	ctx context.Context,
	userID string,
	ids []uuid.UUID,
) error {
	return s.tasks.ReorderTasks(ctx, userID, ids)
}

func (s *TaskService) attachSubtasks(
	ctx context.Context,
	tasks []models.Task,
) ([]models.Task, error) {
	if len(tasks) == 0 {
		return tasks, nil
	}
	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	subtaskMap, err := s.tasks.ListSubtasksForTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		flat := subtaskMap[tasks[i].ID]
		tasks[i].Subtasks = buildSubtaskTree(flat)
		tasks[i].SubtaskTotal = countSubtasksRecursive(tasks[i].Subtasks)
		tasks[i].SubtaskDone = countDoneSubtasksRecursive(tasks[i].Subtasks)
	}
	return tasks, nil
}

func (s *TaskService) attachLinks(
	ctx context.Context,
	tasks []models.Task,
) ([]models.Task, error) {
	if len(tasks) == 0 {
		return tasks, nil
	}
	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	linkMap, err := s.tasks.ListLinksForTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i].Links = linkMap[tasks[i].ID]
	}
	return tasks, nil
}
