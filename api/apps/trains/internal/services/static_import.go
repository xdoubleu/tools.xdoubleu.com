package services

import (
	"context"
	"errors"
	"log/slog"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// importParserVersion identifies what this importer writes. It is stored
// alongside the feed and compared on every run: when the stored rows came
// from an older importer, the conditional-GET validators are dropped so the
// unchanged feed is fetched and imported in full.
//
// Bump this whenever an import writes something it previously did not —
// a new column, a new file parsed, a changed derivation. Version 1 stored a
// single French-only stop name (issue #1453); version 2 was the first to
// fill name_nl/name_fr/name_en from translations.txt (#1450); version 3
// matches those translations by field_value and by unprefixed record_id as
// well, and records the resulting coverage (issue #1459).
const importParserVersion = 3

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
// feed a no-op (issue #1390) — but only while the stored rows come from the
// current importer, since those validators describe the feed and not what
// the importer does with it (issue #1453). A missing BMC key is logged and
// skipped, not an error — matching how games handles a missing
// STEAM_API_KEY.
func (s *StaticImportService) Import(ctx context.Context) error {
	stored, err := s.repos.Feed.GetFeedInfo(ctx)
	if err != nil {
		return err
	}

	//nolint:exhaustruct //both validators optional
	opts := bmc.StaticOptions{}
	switch {
	case stored == nil:
		// Nothing imported yet: an unconditional fetch is the only option.
	case stored.ParserVersion != importParserVersion:
		s.logger.InfoContext(ctx,
			"trains: stored feed predates the current importer, forcing re-import",
			slog.Int("stored_parser_version", stored.ParserVersion),
			slog.Int("parser_version", importParserVersion),
		)
	default:
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
	feed.Info.ParserVersion = importParserVersion

	if err = s.repos.Feed.ImportFeed(ctx, feed); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "trains: static feed imported",
		slog.String("feed_version", feed.Info.FeedVersion),
		slog.Int("parser_version", importParserVersion),
		slog.Int("stops", len(feed.Stops)),
		slog.Int("trips", len(feed.Trips)),
		slog.Int("stop_times", len(feed.StopTimes)),
		slog.Int("calendar_dates", len(feed.CalendarDates)),
		// Coverage, not just counts: an import that reads translations.txt
		// and matches none of it looks identical to a clean one otherwise
		// (issue #1459).
		slog.Int("translation_rows", feed.Info.Translations.Rows),
		slog.Int("translation_rows_unmatched",
			feed.Info.Translations.RowsUnmatched),
		slog.Int("translated_stops_nl", feed.Info.Translations.StopsNL),
		slog.Int("translated_stops_fr", feed.Info.Translations.StopsFR),
		slog.Int("translated_stops_en", feed.Info.Translations.StopsEN),
	)
	return nil
}
