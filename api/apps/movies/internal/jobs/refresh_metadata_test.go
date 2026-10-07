package jobs_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies/internal/jobs"
	"tools.xdoubleu.com/internal/logging"
)

type refresher struct{ err error }

func (r refresher) RefreshCatalog(context.Context, *slog.Logger) error { return r.err }

func TestRefreshMetadataJob(t *testing.T) {
	job := jobs.NewRefreshMetadataJob(refresher{err: nil})
	assert.Equal(t, "movies-refresh-metadata", job.ID())
	assert.Equal(t, 24*time.Hour, job.RunEvery())
	assert.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	failing := jobs.NewRefreshMetadataJob(refresher{err: errors.New("db down")})
	err := failing.Run(context.Background(), logging.NewNopLogger())
	assert.EqualError(t, err, "db down")
}
