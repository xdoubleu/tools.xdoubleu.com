package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type ArchiveJob struct {
	repo *repositories.TasksRepository
}

func NewArchiveJob(repo *repositories.TasksRepository) ArchiveJob {
	return ArchiveJob{repo: repo}
}

func (j ArchiveJob) ID() string {
	return "todos-archive"
}

func (j ArchiveJob) RunEvery() time.Duration {
	return time.Hour
}

func (j ArchiveJob) Run(ctx context.Context, logger *slog.Logger) error {
	ids, err := j.repo.ListDoneForArchiving(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	logger.InfoContext(ctx, "archiving completed tasks", "count", len(ids))
	return j.repo.ArchiveBatch(ctx, ids)
}
