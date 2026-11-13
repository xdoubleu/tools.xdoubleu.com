package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// maxProgressNote is the longest progress note kept; the rest is cut.
const maxProgressNote = 2000

// AddProgressNote comments a running job's progress on its task, as its runner.
// Only the runner that claimed the job may, and only while it runs.
func (s *JobService) AddProgressNote(
	ctx context.Context,
	runnerID, jobID uuid.UUID,
	body string,
) error {
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != models.JobRunning {
		return conflict("the job is no longer running")
	}
	if job.RunnerID == nil || *job.RunnerID != runnerID {
		return forbidden("the job was claimed by another runner")
	}
	body = strings.TrimSpace(strings.ReplaceAll(body, "\x00", ""))
	if body == "" {
		return badRequest("the note is empty")
	}
	if utf8.RuneCountInString(body) > maxProgressNote {
		body = string([]rune(body)[:maxProgressNote])
	}
	_, err = s.tasks.AddComment(ctx, job.TaskID, "runner:"+runnerID.String(), body)
	return err
}
