// Package jobs holds learningpaths' periodic jobs.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

// todoistResyncer is the TodoistService surface the job needs.
type todoistResyncer interface {
	ResyncAll(ctx context.Context) error
}

// TodoistResyncJob reconciles every connected user's Todoist tasks. It runs
// at startup too, so a deploy applies new sync rules to existing paths, and
// hourly, so finishing a linked book frees its slot without a path edit.
type TodoistResyncJob struct {
	todoist todoistResyncer
}

func NewTodoistResyncJob(todoist todoistResyncer) *TodoistResyncJob {
	return &TodoistResyncJob{todoist: todoist}
}

func (j *TodoistResyncJob) ID() string {
	return "learningpaths-todoist-resync"
}

func (j *TodoistResyncJob) RunEvery() time.Duration {
	return time.Hour
}

func (j *TodoistResyncJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.todoist.ResyncAll(ctx)
}
