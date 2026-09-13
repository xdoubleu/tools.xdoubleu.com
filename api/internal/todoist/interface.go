// Package todoist is a minimal client for the parts of Todoist's REST API
// this app needs: creating a task on a user's behalf once they've connected
// their own account via OAuth2 (issue #1475). There is no official Go SDK
// (Todoist ships Python/TypeScript clients only), and this integration is
// one-way and create-only, so a small hand-rolled REST+JSON client is a
// better fit than pulling in an unvetted community package.
package todoist

import "context"

// Client is the narrow surface this app needs from Todoist — task creation
// only, per "no two-way sync" (issue #1471's "Not this"). There is
// deliberately no read/list/update/delete here: nothing server-side ever
// needs to look a created task back up.
type Client interface {
	// CreateTask creates a task with the given content, optionally with a
	// due_string (Todoist's natural-language due-date/recurrence syntax,
	// e.g. "every Monday" — see oauth.go's spike notes; there is no separate
	// structured recurrence field to pass instead). Pass an empty dueString
	// to leave the task undated. Returns Todoist's own task id.
	CreateTask(
		ctx context.Context,
		content, dueString string,
	) (taskID string, err error)
}
