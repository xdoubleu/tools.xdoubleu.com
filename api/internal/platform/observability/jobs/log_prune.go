package jobs

import (
	"context"
	"log/slog"
	"time"
)

const (
	logPruneRunEvery = 24 * time.Hour
	// LogRetention matches get_logs' default 30-day window.
	LogRetention = 30 * 24 * time.Hour
)

type logPruner interface {
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// LogPruneJob keeps global.log_entries to LogRetention.
type LogPruneJob struct {
	repo logPruner
}

func NewLogPruneJob(repo logPruner) *LogPruneJob {
	return &LogPruneJob{repo: repo}
}

func (j *LogPruneJob) ID() string {
	return "prune-log-entries"
}

func (j *LogPruneJob) RunEvery() time.Duration {
	return logPruneRunEvery
}

func (j *LogPruneJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.repo.PruneOlderThan(ctx, time.Now().Add(-LogRetention))
}
