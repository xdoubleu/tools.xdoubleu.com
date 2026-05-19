package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/internal/app"
)

func (s *TaskService) AddSubtask(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	workspaceID *uuid.UUID,
	input string,
	description string,
	parentSubtaskID *uuid.UUID,
) (*models.Subtask, error) {
	if strings.TrimSpace(input) == "" {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Subtask title cannot be empty",
		}
	}
	title, dto := parseQuickInput(input, nil, time.Now())
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(input)
	}
	labels := []string{}
	if dto.Label != "" {
		labels = s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(dto.Label),
		)
	}

	if parentSubtaskID != nil {
		depth, err := s.getSubtaskDepth(ctx, taskID, *parentSubtaskID)
		if err != nil {
			return nil, err
		}
		const maxSubtaskParentDepth = 2
		if depth >= maxSubtaskParentDepth {
			return nil, &app.HTTPError{
				Status:  http.StatusUnprocessableEntity,
				Message: "Maximum subtask depth (3) reached",
			}
		}
	}

	return s.tasks.AddSubtask(
		ctx, taskID, userID,
		title, strings.TrimSpace(description),
		dto.Priority, labels,
		parseDatePtr(dto.DueDate),
		parseDatePtr(dto.Deadline),
		parentSubtaskID,
	)
}

func (s *TaskService) UpdateSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
	workspaceID *uuid.UUID,
	dto dtos.UpdateSubtaskDto,
) (*models.Subtask, error) {
	if strings.TrimSpace(dto.Title) == "" {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Subtask title cannot be empty",
		}
	}
	labels := []string{}
	if dto.Label != "" {
		labels = s.normalizeAndAddLabels(
			ctx, userID, workspaceID, parseLabelsInput(dto.Label),
		)
	}
	return s.tasks.UpdateSubtask(
		ctx, id, taskID, userID,
		strings.TrimSpace(dto.Title),
		strings.TrimSpace(dto.Description),
		dto.Priority, labels,
		parseDatePtr(dto.DueDate),
		parseDatePtr(dto.Deadline),
	)
}

func (s *TaskService) ReorderSubtasks(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	ids []uuid.UUID,
	parentSubtaskID *uuid.UUID,
) error {
	return s.tasks.ReorderSubtasks(ctx, taskID, userID, ids, parentSubtaskID)
}

func (s *TaskService) ToggleSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	return s.tasks.ToggleSubtask(ctx, id, taskID, userID)
}

func (s *TaskService) DeleteSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	return s.tasks.DeleteSubtask(ctx, id, taskID, userID)
}

// getSubtaskDepth retrieves the depth of a subtask in the tree (via repository).
func (s *TaskService) getSubtaskDepth(
	ctx context.Context,
	taskID uuid.UUID,
	subtaskID uuid.UUID,
) (int, error) {
	return s.tasks.GetSubtaskDepth(ctx, taskID, subtaskID)
}

// buildSubtaskTree recursively builds a tree structure from a flat list,
// limiting depth to 3 levels. Only top-level subtasks (ParentSubtaskID == nil)
// are returned with their Children populated.
func buildSubtaskTree(flat []models.Subtask) []models.Subtask {
	const maxDepth = 3
	idToSubtask := make(map[uuid.UUID]*models.Subtask)

	// Build a map for quick lookup
	for i := range flat {
		idToSubtask[flat[i].ID] = &flat[i]
	}

	var result []models.Subtask
	for i := range flat {
		s := &flat[i]
		if s.ParentSubtaskID == nil {
			// Top-level subtask
			populateChildren(s, idToSubtask, 0, maxDepth)
			result = append(result, *s)
		}
	}

	return result
}

// populateChildren recursively populates the Children field of a subtask.
// It stops at maxDepth to prevent deep nesting.
func populateChildren(
	parent *models.Subtask,
	idToSubtask map[uuid.UUID]*models.Subtask,
	currentDepth int,
	maxDepth int,
) {
	if currentDepth >= maxDepth {
		return
	}

	for _, candidate := range idToSubtask {
		if candidate.ParentSubtaskID != nil && *candidate.ParentSubtaskID == parent.ID {
			// Make a copy to avoid shared references
			child := *candidate
			populateChildren(&child, idToSubtask, currentDepth+1, maxDepth)
			parent.Children = append(parent.Children, child)
		}
	}
}

// countSubtasksRecursive counts all subtasks in a tree (including nested ones).
func countSubtasksRecursive(subtasks []models.Subtask) int {
	count := len(subtasks)
	for _, s := range subtasks {
		count += countSubtasksRecursive(s.Children)
	}
	return count
}

// countDoneSubtasksRecursive counts done subtasks in a tree (including nested ones).
func countDoneSubtasksRecursive(subtasks []models.Subtask) int {
	count := 0
	for _, s := range subtasks {
		if s.Done {
			count++
		}
		count += countDoneSubtasksRecursive(s.Children)
	}
	return count
}
