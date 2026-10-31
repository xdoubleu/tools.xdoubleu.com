package services

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/app"
)

const (
	maxTitleLen   = 200
	maxBodyLen    = 20000
	maxCommentLen = 10000
	maxLabels     = 10
	maxLabelLen   = 40
)

type TaskService struct {
	repo *repositories.TasksRepository
}

// TaskInput is the editable part of a task.
type TaskInput struct {
	Title    string
	Body     string
	Priority models.Priority
	Labels   []string
}

func badRequest(msg string) error {
	return &app.HTTPError{Status: http.StatusBadRequest, Message: msg}
}

func forbidden(msg string) error {
	return &app.HTTPError{Status: http.StatusForbidden, Message: msg}
}

// normalise trims and validates in, defaulting an unset priority to P2.
func normalise(in TaskInput) (TaskInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	switch {
	case in.Title == "":
		return in, badRequest("title is required")
	case utf8.RuneCountInString(in.Title) > maxTitleLen:
		return in, badRequest("title is too long")
	case utf8.RuneCountInString(in.Body) > maxBodyLen:
		return in, badRequest("body is too long")
	}
	if in.Priority == "" {
		in.Priority = models.PriorityP2
	}
	if !in.Priority.Valid() {
		return in, badRequest("invalid priority")
	}

	labels := []string{}
	seen := map[string]bool{}
	for _, l := range in.Labels {
		l = strings.TrimSpace(l)
		if l == "" || seen[l] {
			continue
		}
		if utf8.RuneCountInString(l) > maxLabelLen {
			return in, badRequest("label is too long")
		}
		seen[l] = true
		labels = append(labels, l)
	}
	if len(labels) > maxLabels {
		return in, badRequest("too many labels")
	}
	in.Labels = labels
	return in, nil
}

func normaliseComment(body string) (string, error) {
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return "", badRequest("comment is required")
	case utf8.RuneCountInString(body) > maxCommentLen:
		return "", badRequest("comment is too long")
	}
	return body, nil
}

// List returns every task without its body.
func (s *TaskService) List(ctx context.Context) ([]models.Task, error) {
	return s.repo.List(ctx)
}

// Get returns a task with its comments.
func (s *TaskService) Get(
	ctx context.Context,
	id uuid.UUID,
) (*models.Task, []models.Comment, error) {
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	comments, err := s.repo.Comments(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return task, comments, nil
}

// Create adds a task to Backlog.
func (s *TaskService) Create(
	ctx context.Context,
	actor models.Actor,
	in TaskInput,
) (*models.Task, error) {
	in, err := normalise(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, models.Task{
		ID:           uuid.Nil,
		Title:        in.Title,
		Body:         in.Body,
		Status:       models.StatusBacklog,
		Priority:     in.Priority,
		Labels:       in.Labels,
		CreatedBy:    "",
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
		CommentCount: 0,
	}, actor)
}

// Update replaces a task's title, body, priority and labels.
func (s *TaskService) Update(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	in TaskInput,
) (*models.Task, error) {
	in, err := normalise(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, models.Task{
		ID:           id,
		Title:        in.Title,
		Body:         in.Body,
		Status:       "",
		Priority:     in.Priority,
		Labels:       in.Labels,
		CreatedBy:    "",
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
		CommentCount: 0,
	}, actor)
}

// checkMove rejects an unknown column, and Ready for anyone but a human admin
// in a browser session.
func checkMove(actor models.Actor, status models.Status) error {
	if !status.Valid() {
		return badRequest("invalid status")
	}
	if status == models.StatusReady && !actor.CanMoveToReady() {
		return forbidden("only a human in a browser session can mark a task Ready")
	}
	return nil
}

// Move changes a task's column. Only a human admin in a browser session may
// move a task into Ready.
func (s *TaskService) Move(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	status models.Status,
) (*models.Task, error) {
	if err := checkMove(actor, status); err != nil {
		return nil, err
	}
	return s.repo.Move(ctx, id, status)
}

// Comment adds a comment to a task.
func (s *TaskService) Comment(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	body string,
) (*models.Comment, error) {
	body, err := normaliseComment(body)
	if err != nil {
		return nil, err
	}
	return s.repo.AddComment(ctx, id, actor.String(), body)
}
