package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/learningpaths/internal/jobs"
	"tools.xdoubleu.com/internal/kit/logging"
)

type fakeResyncer struct {
	calls int
	err   error
}

func (f *fakeResyncer) ResyncAll(_ context.Context) error {
	f.calls++
	return f.err
}

func TestTodoistResyncJob(t *testing.T) {
	want := errors.New("boom")
	fake := &fakeResyncer{calls: 0, err: want}
	job := jobs.NewTodoistResyncJob(fake)

	assert.Equal(t, "learningpaths-todoist-resync", job.ID())
	assert.Equal(t, time.Hour, job.RunEvery())
	assert.ErrorIs(t, job.Run(t.Context(), logging.NewNopLogger()), want)
	assert.Equal(t, 1, fake.calls)
}
