package services

import (
	"context"
	"errors"
	"log/slog"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// StaticImportService downloads, validates and imports the SNCB GTFS static
// timetable into the trains schema. It is driven by jobs.StaticImportJob on
// a 24h cadence.
type StaticImportService struct {
	logger *slog.Logger
	repos  *repositories.Repositories
	bmc    bmc.Client
}

func NewStaticImportService(
	logger *slog.Logger,
	repos *repositories.Repositories,
	bmcClient bmc.Client,
) *StaticImportService {
	return &StaticImportService{logger: logger, repos: repos, bmc: bmcClient}
}

// Import runs one import cycle. A conditional GET makes an unchanged daily
// feed a no-op (issue #1390). A missing BMC key is logged and skipped, not
// an error — matching how games handles a missing STEAM_API_KEY.
func (s *StaticImportService) Import(ctx context.Context) error {
	stored, err := s.repos.Feed.GetFeedInfo(ctx)
	if err != nil {
		return err
	}

	//nolint:exhaustruct //both validators optional
	opts := bmc.StaticOptions{}
	if stored != nil {
		opts.ETag = stored.ETag
		opts.LastModified = stored.LastModified
	}

	res, err := s.bmc.FetchStatic(ctx, opts)
	if errors.Is(err, bmc.ErrNotConfigured) {
		s.logger.WarnContext(ctx,
			"trains: BMC_PARTNER_KEY not set — static import skipped")
		return nil
	}
	if err != nil {
		return err
	}
	if res.NotModified {
		s.logger.InfoContext(ctx, "trains: static feed unchanged, import skipped")
		return nil
	}

	feed, err := parseFeed(s.logger, res.Body)
	if err != nil {
		return err
	}
	feed.Info.ETag = res.ETag
	feed.Info.LastModified = res.LastModified

	if err = s.repos.Feed.ImportFeed(ctx, feed); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "trains: static feed imported",
		slog.String("feed_version", feed.Info.FeedVersion),
		slog.Int("stops", len(feed.Stops)),
		slog.Int("trips", len(feed.Trips)),
		slog.Int("stop_times", len(feed.StopTimes)),
		slog.Int("calendar_dates", len(feed.CalendarDates)),
	)
	return nil
}
