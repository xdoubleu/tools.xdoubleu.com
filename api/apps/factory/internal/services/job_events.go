package services

import (
	"context"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
)

const (
	// EventRetention is how long a job's log is kept.
	EventRetention = 30 * 24 * time.Hour

	maxEventsPerCall = 200
	maxEventsPerJob  = 5000
	maxEventBytes    = 32 * 1024
	maxKindLen       = 40
	// ListEventsLimit is the most events one ListEvents call returns.
	ListEventsLimit = 500
	rawKind         = "raw"
)

// kindOf is a stream-json line's `type`, or "raw" for anything else.
func kindOf(line string) string {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal([]byte(line), &head) != nil || head.Type == "" ||
		len(head.Type) > maxKindLen {
		return rawKind
	}
	return head.Type
}

// truncate keeps the first maxEventBytes of a line, on a rune boundary.
func truncate(line string) string {
	if len(line) <= maxEventBytes {
		return line
	}
	cut := line[:maxEventBytes]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// AppendEvents adds a batch of the harness's output lines to a running job's
// log. The runner has already removed credential values. A job that is not
// running takes none, so a stopped job's runner stops being heard.
func (s *JobService) AppendEvents(
	ctx context.Context,
	jobID uuid.UUID,
	lines []string,
) error {
	if len(lines) > maxEventsPerCall {
		return badRequest("too many events in one call")
	}
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != models.JobRunning {
		return conflict("the job is no longer running")
	}
	kinds := make([]string, 0, len(lines))
	payloads := make([]string, 0, len(lines))
	for _, line := range lines {
		kinds = append(kinds, kindOf(line))
		payloads = append(payloads, truncate(line))
	}
	return s.events.Append(ctx, jobID, kinds, payloads, maxEventsPerJob)
}

// ListEvents is a job's log after afterSeq, for a human admin in a browser: it
// can hold code and file contents.
func (s *JobService) ListEvents(
	ctx context.Context,
	actor models.Actor,
	jobID uuid.UUID,
	afterSeq int,
) ([]models.JobEvent, error) {
	if !actor.CanMoveToReady() {
		return nil, forbidden("only a human admin in a browser session can read a job log")
	}
	if _, err := s.jobs.Get(ctx, jobID); err != nil {
		return nil, err
	}
	return s.events.After(ctx, jobID, max(afterSeq, 0), ListEventsLimit)
}

// PruneEvents deletes logs older than EventRetention.
func (s *JobService) PruneEvents(ctx context.Context) (int64, error) {
	return s.events.DeleteBefore(ctx, s.now().Add(-EventRetention))
}
