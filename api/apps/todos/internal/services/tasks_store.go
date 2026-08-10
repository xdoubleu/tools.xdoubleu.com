package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

// tasksRepo is the storage surface TaskService needs. It is satisfied by
// repositories.TasksRepository and by fakes in unit tests, so lifecycle,
// listing, and search rules can be tested without a database.
type tasksRepo interface {
	GetByID(ctx context.Context, id uuid.UUID, userID string) (*models.Task, error)
	Create(ctx context.Context, t models.Task) (*models.Task, error)
	Update(ctx context.Context, t models.Task) error
	MoveSection(
		ctx context.Context,
		id uuid.UUID,
		userID string,
		sectionID *uuid.UUID,
	) error
	Delete(ctx context.Context, id uuid.UUID, userID string) error
	SetStatus(
		ctx context.Context,
		id uuid.UUID,
		userID string,
		status string,
		completedAt *time.Time,
		archivedAt *time.Time,
	) error
	ReorderTasks(ctx context.Context, userID string, ids []uuid.UUID) error

	ListOpen(
		ctx context.Context,
		userID string,
		sectionID *uuid.UUID,
		workspaceID *uuid.UUID,
		limit int32,
		offset int32,
	) ([]models.Task, bool, error)
	CountOpenPerSection(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) (map[string]int, error)
	ListByStatus(
		ctx context.Context,
		userID string,
		status string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
	ListArchived(
		ctx context.Context,
		userID string,
		query string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
	SearchAll(
		ctx context.Context,
		userID string,
		query string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
	SearchByLinkURL(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
		linkURL string,
	) ([]models.Task, error)
	ReplaceLinks(ctx context.Context, taskID uuid.UUID, links []models.TaskLink) error
	ListLinksForTasks(
		ctx context.Context,
		taskIDs []uuid.UUID,
	) (map[uuid.UUID][]models.TaskLink, error)

	CreateSubtask(
		ctx context.Context,
		taskID uuid.UUID,
		userID string,
		title string,
		description string,
		priority int,
		labels []string,
		dueDate *time.Time,
		deadline *time.Time,
		parentSubtaskID *uuid.UUID,
	) (*models.Subtask, error)
	UpdateSubtask(
		ctx context.Context,
		id uuid.UUID,
		taskID uuid.UUID,
		userID string,
		title string,
		description string,
		priority int,
		labels []string,
		dueDate *time.Time,
		deadline *time.Time,
	) (*models.Subtask, error)
	ToggleSubtask(
		ctx context.Context,
		id uuid.UUID,
		taskID uuid.UUID,
		userID string,
	) error
	DeleteSubtask(
		ctx context.Context,
		id uuid.UUID,
		taskID uuid.UUID,
		userID string,
	) error
	GetSubtaskDepth(
		ctx context.Context,
		taskID uuid.UUID,
		subtaskID uuid.UUID,
	) (int, error)
	ReorderSubtasks(
		ctx context.Context,
		taskID uuid.UUID,
		userID string,
		ids []uuid.UUID,
		parentSubtaskID *uuid.UUID,
	) error
	ListSubtasksForTasks(
		ctx context.Context,
		taskIDs []uuid.UUID,
	) (map[uuid.UUID][]models.Subtask, error)
}
