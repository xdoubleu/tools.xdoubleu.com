// Package todoist is a minimal hand-rolled client for creating and deleting
// Todoist tasks on a user's behalf (no official Go SDK).
package todoist

import "context"

// Client is a one-way outbox: it creates and deletes tasks, but a completed
// task never flips the owning learning-path item.
type Client interface {
	// CreateTask creates a task and returns its id. dueString is Todoist's
	// natural-language due/recurrence syntax (e.g. "every Monday"); empty leaves
	// it undated.
	CreateTask(
		ctx context.Context,
		content, dueString string,
	) (taskID string, err error)

	// DeleteTask deletes a task by id. A task Todoist no longer knows (already
	// completed or deleted) is not an error — idempotent callers rely on that.
	DeleteTask(ctx context.Context, taskID string) (err error)
}
