package jobs

import (
	"context"
	"log/slog"
	"time"
)

// catalogRefresher is the slice of services.MovieService the job needs.
type catalogRefresher interface {
	RefreshCatalog(ctx context.Context, logger *slog.Logger) error
}

// RefreshMetadataJob keeps the TMDB catalog fresh and within its cache limit.
type RefreshMetadataJob struct {
	movies catalogRefresher
}

func NewRefreshMetadataJob(movies catalogRefresher) RefreshMetadataJob {
	return RefreshMetadataJob{movies: movies}
}

func (j RefreshMetadataJob) ID() string {
	return "movies-refresh-metadata"
}

func (j RefreshMetadataJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j RefreshMetadataJob) Run(ctx context.Context, logger *slog.Logger) error {
	return j.movies.RefreshCatalog(ctx, logger)
}
