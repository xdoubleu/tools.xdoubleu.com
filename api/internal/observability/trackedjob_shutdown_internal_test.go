package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
)

// ctxInserter fails on a cancelled ctx, as the real repository would.
type ctxInserter struct {
	runs []models.JobRun
}

func (f *ctxInserter) Insert(ctx context.Context, run models.JobRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.runs = append(f.runs, run)
	return nil
}

func TestTrackedJobRun_ShutdownRecordsTheInterruptedRunAtWarn(t *testing.T) {
	repo := &ctxInserter{runs: nil}
	job := &TrackedJob{
		inner: blockingJob{deadline: time.Hour, swallow: false},
		repo:  repo,
	}
	var buf bytes.Buffer
	logger := slog.New(logging.NewBufLogHandler(&buf, nil))

	rootCtx, cancelRoot := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- job.Run(rootCtx, logger) }()
	cancelRoot()

	require.ErrorIs(t, <-done, context.Canceled)
	require.Len(t, repo.runs, 1)
	assert.False(t, repo.runs[0].Success)
	assert.Contains(t, buf.String(), "level=WARN")
	assert.NotContains(t, buf.String(), "level=ERROR")
}
