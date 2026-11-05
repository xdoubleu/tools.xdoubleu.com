package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// maxSessionQuery is the most task ids one ListFinishedTasks call may carry.
const maxSessionQuery = 500

// CleanSessions deletes the session state of tasks the server reports as Done or
// gone. The server is untrusted, so it can only name directories the runner asked
// about: each is a UUID-named directory it found itself. A failure is only logged;
// the next job tries again.
func (r *JobRunner) CleanSessions(ctx context.Context) {
	root := filepath.Join(r.DataDir, "sessions")
	ids := r.sessionTasks(root)
	for len(ids) > 0 {
		batch := ids[:min(len(ids), maxSessionQuery)]
		ids = ids[len(batch):]
		finished, err := r.API.ListFinishedTasks(ctx, r.Token, batch)
		if err != nil {
			if ctx.Err() == nil {
				r.Log.Warn("could not ask which tasks are finished", "error", err)
			}
			return
		}
		r.deleteSessions(root, batch, finished)
	}
}

// sessionTasks are the task ids that have a session directory under root.
func (r *JobRunner) sessionTasks(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			r.Log.Warn("could not list the session directories", "error", err)
		}
		return nil
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() && jobIDPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	return ids
}

// deleteSessions removes the finished tasks' directories, but only those the
// runner asked about.
func (r *JobRunner) deleteSessions(root string, asked, finished []string) {
	for _, id := range finished {
		if !slices.Contains(asked, id) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
			r.Log.Warn("could not delete a session directory", "task", id, "error", err)
		}
	}
}
