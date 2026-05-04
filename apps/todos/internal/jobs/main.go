package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
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
	tasks, err := j.repo.ListDoneForArchiving(ctx)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}

	logger.InfoContext(ctx, "archiving completed tasks", "count", len(ids))
	return j.repo.ArchiveBatch(ctx, ids)
}
