// Package todoist is a minimal hand-rolled client for creating and deleting
// Todoist tasks, projects and sections on a user's behalf (no official Go SDK).
package todoist

import "context"

// Client is a one-way outbox: it creates and deletes tasks, but a completed
// task never flips the owning learning-path item.
type Client interface {
	// CreateTask creates a task and returns its id. dueString is Todoist's
	// natural-language due/recurrence syntax (e.g. "every Monday"); empty leaves
	// it undated. An empty projectID puts it in the Inbox; an empty sectionID
	// leaves it outside any section.
	CreateTask(
		ctx context.Context,
		content, dueString, projectID, sectionID string,
	) (taskID string, err error)

	// DeleteTask deletes a task by id. A task Todoist no longer knows (already
	// completed or deleted) is not an error — idempotent callers rely on that.
	DeleteTask(ctx context.Context, taskID string) (err error)

	// CreateProject creates a project and returns its id.
	CreateProject(ctx context.Context, name string) (projectID string, err error)

	// ProjectExists reports whether projectID still exists.
	ProjectExists(ctx context.Context, projectID string) (bool, error)

	// DeleteProject deletes a project and its tasks; an unknown project is not
	// an error.
	DeleteProject(ctx context.Context, projectID string) error

	// CreateSection creates a section in projectID and returns its id.
	CreateSection(
		ctx context.Context, projectID, name string,
	) (sectionID string, err error)

	// SectionExists reports whether sectionID still exists.
	SectionExists(ctx context.Context, sectionID string) (bool, error)

	// DeleteSection deletes a section and its tasks; an unknown section is not
	// an error.
	DeleteSection(ctx context.Context, sectionID string) error
}
