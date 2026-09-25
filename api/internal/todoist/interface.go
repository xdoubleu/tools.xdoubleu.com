// Package todoist is a minimal hand-rolled client for creating Todoist tasks
// on a user's behalf (no official Go SDK).
package todoist

import "context"

// Client is create-only: there is no two-way sync.
type Client interface {
	// CreateTask creates a task and returns its id. dueString is Todoist's
	// natural-language due/recurrence syntax (e.g. "every Monday"); empty leaves
	// it undated.
	CreateTask(
		ctx context.Context,
		content, dueString string,
	) (taskID string, err error)
}
