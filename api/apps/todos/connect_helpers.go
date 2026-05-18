package todos

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/todos/internal/models"
	todosv1 "tools.xdoubleu.com/gen/todos/v1"
	"tools.xdoubleu.com/gen/todos/v1/todosv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// ── TaskService handler ───────────────────────────────────────────────────────

type taskConnectHandler struct {
	app *Todos
}

var _ todosv1connect.TaskServiceHandler = (*taskConnectHandler)(nil)

// ── SettingsService handler ───────────────────────────────────────────────────

type settingsConnectHandler struct {
	app *Todos
}

var _ todosv1connect.SettingsServiceHandler = (*settingsConnectHandler)(nil)

// ── SubtaskService handler ────────────────────────────────────────────────────

type subtaskConnectHandler struct {
	app *Todos
}

var _ todosv1connect.SubtaskServiceHandler = (*subtaskConnectHandler)(nil)

// ── Shared helpers ────────────────────────────────────────────────────────────

func (h *taskConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	return u.ID
}

func (h *subtaskConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	return u.ID
}

// ── proto conversion helpers ──────────────────────────────────────────────────

func protoTask(t models.Task) *todosv1.Task {
	links := make([]*todosv1.TaskLink, len(t.Links))
	for i, l := range t.Links {
		links[i] = &todosv1.TaskLink{
			Id:     l.ID.String(),
			TaskId: l.TaskID.String(),
			Url:    l.URL,
			Label:  l.Label,
			SortOrder: int32( //nolint:gosec // int32 safe for domain values
				l.SortOrder,
			),
			ShortcutBadge: l.ShortcutBadge,
		}
	}
	subtasks := make([]*todosv1.Subtask, len(t.Subtasks))
	for i, s := range t.Subtasks {
		subtasks[i] = protoSubtask(s)
	}
	return &todosv1.Task{
		Id:          t.ID.String(),
		OwnerUserId: t.OwnerUserID,
		Title:       t.Title,
		Description: t.Description,
		Labels:      t.Labels,
		Status:      t.Status,
		Priority:    int32(t.Priority),  //nolint:gosec // int32 safe for domain values
		SortOrder:   int32(t.SortOrder), //nolint:gosec // int32 safe for domain values
		CompletedAt: timeToStr(t.CompletedAt),
		ArchivedAt:  timeToStr(t.ArchivedAt),
		DueDate:     datePtrToStr(t.DueDate),
		Deadline:    timePtrToRFC3339(t.Deadline),
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
		SectionId:   uuidPtrToStr(t.SectionID),
		WorkspaceId: uuidPtrToStr(t.WorkspaceID),
		RecurDays:   int32(t.RecurDays), //nolint:gosec // int32 safe for domain values
		RecurRule:   t.RecurRule,
		Links:       links,
		Subtasks:    subtasks,
		SubtaskDone: int32( //nolint:gosec // int32 safe for domain values
			t.SubtaskDone,
		),
		SubtaskTotal: int32( //nolint:gosec // int32 safe for domain values
			t.SubtaskTotal,
		),
	}
}

func protoSubtask(s models.Subtask) *todosv1.Subtask {
	children := make([]*todosv1.Subtask, len(s.Children))
	for i, ch := range s.Children {
		children[i] = protoSubtask(ch)
	}
	return &todosv1.Subtask{
		Id:          s.ID.String(),
		TaskId:      s.TaskID.String(),
		Title:       s.Title,
		Description: s.Description,
		Done:        s.Done,
		SortOrder: int32( //nolint:gosec // int32 safe for domain values
			s.SortOrder,
		),
		Priority: int32( //nolint:gosec // int32 safe for domain values
			s.Priority,
		),
		Labels:          s.Labels,
		DueDate:         datePtrToStr(s.DueDate),
		Deadline:        timePtrToRFC3339(s.Deadline),
		CreatedAt:       s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       s.UpdatedAt.Format(time.RFC3339),
		ParentSubtaskId: uuidPtrToStr(s.ParentSubtaskID),
		Children:        children,
	}
}

func protoTasks(tasks []models.Task) []*todosv1.Task {
	out := make([]*todosv1.Task, len(tasks))
	for i, t := range tasks {
		out[i] = protoTask(t)
	}
	return out
}

func protoSections(sections []models.Section) []*todosv1.Section {
	out := make([]*todosv1.Section, len(sections))
	for i, s := range sections {
		out[i] = &todosv1.Section{
			Id:          s.ID.String(),
			OwnerUserId: s.OwnerUserID,
			Name:        s.Name,
			SortOrder: int32( //nolint:gosec // int32 safe for domain values
				s.SortOrder,
			),
			CreatedAt:   s.CreatedAt.Format(time.RFC3339),
			WorkspaceId: uuidPtrToStr(s.WorkspaceID),
		}
	}
	return out
}

func protoWorkspaces(workspaces []models.Workspace) []*todosv1.Workspace {
	out := make([]*todosv1.Workspace, len(workspaces))
	for i, w := range workspaces {
		out[i] = &todosv1.Workspace{
			Id:          w.ID.String(),
			OwnerUserId: w.OwnerUserID,
			Name:        w.Name,
			CreatedAt:   w.CreatedAt.Format(time.RFC3339),
		}
	}
	return out
}

func uuidPtrToStr(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func timeToStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func datePtrToStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func timePtrToRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// connectErr maps service errors to ConnectRPC error codes.
func connectErr(err error) error {
	if err == nil {
		return nil
	}
	return connect.NewError(connect.CodeInternal, err)
}
