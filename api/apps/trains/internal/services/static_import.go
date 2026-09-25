package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
	"tools.xdoubleu.com/internal/observability"
)

// StaticImportJobID is jobs.StaticImportJob's ID, kept next to the phase
// metrics that report under it so they can't drift.
const StaticImportJobID = "trains-static-import"

// Phases reported via observability.ObserveJobPhase so prom_query can
// attribute a slow import to fetch, parse, or DB import.
const (
	phaseFetch  = "fetch"
	phaseParse  = "parse"
	phaseImport = "import"
)

// ImportParserVersion identifies what this importer writes. When stored rows
// come from an older version, the conditional-GET validators are dropped and
// the feed is reimported. Bump it whenever the import writes something new
// (a column, a file, a changed derivation).
const ImportParserVersion = 5

// StaticImportService fetches, validates and imports the SNCB GTFS static
// timetable; driven daily by jobs.StaticImportJob.
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

// Import runs one cycle. A conditional GET skips an unchanged feed, but only
// if the stored rows come from the current ImportParserVersion. A missing BMC
// key is logged and skipped.
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
	case stored.ParserVersion != ImportParserVersion:
		s.logger.InfoContext(ctx,
			"trains: stored feed predates the current importer, forcing re-import",
			slog.Int("stored_parser_version", stored.ParserVersion),
			slog.Int("parser_version", ImportParserVersion),
		)
	default:
		opts.ETag = stored.ETag
		opts.LastModified = stored.LastModified
	}

	fetchStart := time.Now()
	res, err := s.bmc.FetchStatic(ctx, opts)
	fetchDur := time.Since(fetchStart)
	observability.ObserveJobPhase(StaticImportJobID, phaseFetch, fetchDur)
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

	parseStart := time.Now()
	feed, err := parseFeed(s.logger, res.Body)
	parseDur := time.Since(parseStart)
	observability.ObserveJobPhase(StaticImportJobID, phaseParse, parseDur)
	if err != nil {
		return err
	}
	feed.Info.ETag = res.ETag
	feed.Info.LastModified = res.LastModified
	feed.Info.ParserVersion = ImportParserVersion

	importStart := time.Now()
	err = s.repos.Feed.ImportFeed(ctx, feed)
	importDur := time.Since(importStart)
	observability.ObserveJobPhase(StaticImportJobID, phaseImport, importDur)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "trains: static import phase durations",
		slog.String("job", StaticImportJobID),
		slog.Duration("fetch", fetchDur),
		slog.Duration("parse", parseDur),
		slog.Duration("import", importDur),
	)

	s.logger.InfoContext(ctx, "trains: static feed imported",
		slog.String("feed_version", feed.Info.FeedVersion),
		slog.Int("parser_version", ImportParserVersion),
		slog.Int("stops", len(feed.Stops)),
		slog.Int("trips", len(feed.Trips)),
		slog.Int("stop_times", len(feed.StopTimes)),
		slog.Int("calendar_dates", len(feed.CalendarDates)),
		// Log coverage: an import matching no translations otherwise looks clean.
		slog.Int("translation_rows", feed.Info.Translations.Rows),
		slog.Int("translation_rows_unmatched",
			feed.Info.Translations.RowsUnmatched),
		slog.Int("translated_stops_nl", feed.Info.Translations.StopsNL),
		slog.Int("translated_stops_fr", feed.Info.Translations.StopsFR),
		slog.Int("translated_stops_en", feed.Info.Translations.StopsEN),
	)
	return nil
}
