package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

const schemaName = "trains"

type FeedRepository struct {
	db postgres.DB
}

// GetFeedInfo returns the stored feed metadata, or (nil, nil) when nothing
// has been imported yet.
func (r *FeedRepository) GetFeedInfo(
	ctx context.Context,
) (*models.FeedInfo, error) {
	//nolint:exhaustruct //scan target
	info := &models.FeedInfo{}
	var lang, etag, lastModified *string
	err := r.db.QueryRow(ctx, `
		SELECT feed_version, feed_start_date, feed_end_date, feed_lang,
		       etag, last_modified, imported_at, parser_version,
		       translated_stops_nl, translated_stops_fr, translated_stops_en,
		       translation_rows, translation_rows_unmatched
		FROM trains.feed_info
		WHERE singleton
	`).Scan(
		&info.FeedVersion, &info.StartDate, &info.EndDate, &lang,
		&etag, &lastModified, &info.ImportedAt, &info.ParserVersion,
		&info.Translations.StopsNL, &info.Translations.StopsFR,
		&info.Translations.StopsEN, &info.Translations.Rows,
		&info.Translations.RowsUnmatched,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil //"nothing imported yet" is a valid state
	}
	if err != nil {
		return nil, err
	}
	if lang != nil {
		info.Lang = *lang
	}
	if etag != nil {
		info.ETag = *etag
	}
	if lastModified != nil {
		info.LastModified = *lastModified
	}
	return info, nil
}

// stagedTables is the ordered set of bare table names ImportFeed replaces —
// each one has a live `trains.<name>` table and a `trains.<name>_staging`
// mirror (identical columns/PK/indexes, migration 00008_staging_tables.sql).
//
//nolint:gochecknoglobals //fixed table list, package-level by design
var stagedTables = []string{
	"stop_times", "calendar_dates", "transfers", "trips", "routes", "stops",
}

// ImportFeed replaces the entire trains timetable with feed. It runs in two
// phases, never taking a lock on a live table for longer than a metadata-only
// rename: PopulateStaging does the slow TRUNCATE + COPY entirely against the
// `_staging` tables, which nothing else ever queries, then SwapStagingIn
// renames each staging table into its live counterpart's name. A single
// TRUNCATE-then-COPY transaction against the live tables directly (the
// previous approach) held Postgres's TRUNCATE ACCESS EXCLUSIVE lock — which,
// unlike UPDATE/DELETE, conflicts with even a plain SELECT — for the whole
// multi-minute import, blocking every concurrent read of those tables
// (issue #1718).
func (r *FeedRepository) ImportFeed(
	ctx context.Context,
	feed *models.Feed,
) error {
	if err := r.PopulateStaging(ctx, feed); err != nil {
		return err
	}
	return r.SwapStagingIn(ctx, feed.Info)
}

// PopulateStaging truncates and repopulates the `_staging` tables with feed,
// entirely independent of the live tables — safe to run for as long as the
// import takes without affecting any concurrent reader.
func (r *FeedRepository) PopulateStaging(
	ctx context.Context,
	feed *models.Feed,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	truncateList := make([]string, len(stagedTables))
	for i, t := range stagedTables {
		truncateList[i] = fmt.Sprintf("trains.%s_staging", t)
	}

	ctx, span := startStep(ctx, "db.trains.truncate_staging")
	_, err = tx.Exec(
		ctx, fmt.Sprintf("TRUNCATE %s", strings.Join(truncateList, ", ")),
	)
	span.Finish()
	if err != nil {
		return err
	}

	if err = copyTable(ctx, tx, "stops", feed.Stops, copyStops); err != nil {
		return err
	}
	if err = copyTable(ctx, tx, "routes", feed.Routes, copyRoutes); err != nil {
		return err
	}
	if err = copyTable(ctx, tx, "trips", feed.Trips, copyTrips); err != nil {
		return err
	}
	if err = copyTable(ctx, tx, "stop_times", feed.StopTimes, copyStopTimes); err != nil {
		return err
	}
	if err = copyTable(
		ctx, tx, "calendar_dates", feed.CalendarDates, copyCalendarDates,
	); err != nil {
		return err
	}
	if err = copyTable(ctx, tx, "transfers", feed.Transfers, copyTransfers); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// startStep opens a Sentry span for one import step, so each step's
// duration is attributed inside the trains-static-import Sentry transaction
// the job already reports under (issue #1818). sentry.StartSpan attaches as
// a child of whatever span the context carries; off a job's transaction (or
// without Sentry initialized) the span is inert, so untraced callers such as
// integration tests pay nothing.
func startStep(ctx context.Context, op string) (context.Context, *sentry.Span) {
	span := sentry.StartSpan(ctx, op)
	return span.Context(), span
}

// copyTable runs one table's COPY under its own Sentry child span. The
// generic is the row slice type; fn performs the actual CopyFrom.
func copyTable[Rows any](
	ctx context.Context,
	tx pgx.Tx,
	table string,
	rows []Rows,
	fn func(context.Context, pgx.Tx, []Rows) error,
) error {
	spanCtx, span := startStep(ctx, "db.trains.copy."+table)
	err := fn(spanCtx, tx, rows)
	span.Finish()
	return err
}

// SwapStagingIn atomically promotes the populated `_staging` tables to be
// the live tables, and demotes the previous live tables back to `_staging`
// (PopulateStaging truncates them on the next import). Each table is swapped
// via a 3-way rename — live→tmp, staging→live, tmp→staging — which is
// metadata-only and needs an ACCESS EXCLUSIVE lock on the live table for
// only as long as the rename itself takes, not the size of the data it
// carries. info is written in the same transaction so a reader never
// observes new timetable rows against stale feed_info metadata.
func (r *FeedRepository) SwapStagingIn(
	ctx context.Context,
	info models.FeedInfo,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ctx, span := startStep(ctx, "db.trains.swap_staging")
	defer span.Finish()

	for _, t := range stagedTables {
		tmp := t + "_swap_tmp"
		if _, err = tx.Exec(
			ctx, fmt.Sprintf("ALTER TABLE trains.%s RENAME TO %s", t, tmp),
		); err != nil {
			return err
		}
		if _, err = tx.Exec(
			ctx,
			fmt.Sprintf("ALTER TABLE trains.%s_staging RENAME TO %s", t, t),
		); err != nil {
			return err
		}
		if _, err = tx.Exec(
			ctx,
			fmt.Sprintf("ALTER TABLE trains.%s RENAME TO %s_staging", tmp, t),
		); err != nil {
			return err
		}
	}

	if err = upsertFeedInfo(ctx, tx, info); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CountTripsResolvingOn returns how many trips have a service that runs on
// date, resolved from calendar_dates alone — the assertion that catches the
// calendar.txt decoy trap (issue #1390).
func (r *FeedRepository) CountTripsResolvingOn(
	ctx context.Context,
	date time.Time,
) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM trains.trips t
		WHERE EXISTS (
			SELECT 1 FROM trains.calendar_dates cd
			WHERE cd.service_id = t.service_id
			  AND cd.date = $1
			  AND cd.exception_type = 1
		)
	`, date).Scan(&count)
	return count, err
}

func copyStops(ctx context.Context, tx pgx.Tx, rows []models.Stop) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "stops_staging"},
		[]string{
			"stop_id", "parent_station", "name_nl", "name_fr", "name_en",
			"display_name", "location_type", "platform_code", "uic", "lat", "lon",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			s := rows[i]
			return []any{
				s.StopID, nullStr(s.ParentStation), s.NameNL, s.NameFR, s.NameEN,
				s.DisplayName, s.LocationType, nullStr(s.PlatformCode), nullStr(s.UIC),
				s.Lat, s.Lon,
			}, nil
		}),
	)
	return err
}

func copyRoutes(ctx context.Context, tx pgx.Tx, rows []models.Route) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "routes_staging"},
		[]string{"route_id", "short_name", "long_name", "route_type"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			r := rows[i]
			return []any{
				r.RouteID, nullStr(r.ShortName), nullStr(r.LongName), r.RouteType,
			}, nil
		}),
	)
	return err
}

func copyTrips(ctx context.Context, tx pgx.Tx, rows []models.Trip) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "trips_staging"},
		[]string{
			"trip_id", "route_id", "service_id", "trip_short_name",
			"trip_headsign", "direction_id",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			t := rows[i]
			return []any{
				t.TripID, t.RouteID, t.ServiceID, nullStr(t.ShortName),
				nullStr(t.Headsign), t.DirectionID,
			}, nil
		}),
	)
	return err
}

func copyStopTimes(ctx context.Context, tx pgx.Tx, rows []models.StopTime) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "stop_times_staging"},
		[]string{
			"trip_id", "stop_sequence", "stop_id", "arrival_seconds",
			"departure_seconds", "pickup_type", "drop_off_type",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			st := rows[i]
			return []any{
				st.TripID, st.StopSequence, st.StopID, st.ArrivalSeconds,
				st.DepartureSeconds, st.PickupType, st.DropOffType,
			}, nil
		}),
	)
	return err
}

func copyCalendarDates(
	ctx context.Context, tx pgx.Tx, rows []models.CalendarDate,
) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "calendar_dates_staging"},
		[]string{"service_id", "date", "exception_type"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			cd := rows[i]
			return []any{cd.ServiceID, cd.Date, cd.ExceptionType}, nil
		}),
	)
	return err
}

func copyTransfers(ctx context.Context, tx pgx.Tx, rows []models.Transfer) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "transfers_staging"},
		[]string{
			"from_stop_id", "to_stop_id", "transfer_type", "min_transfer_time",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			t := rows[i]
			return []any{
				t.FromStopID, t.ToStopID, t.TransferType, t.MinTransferTime,
			}, nil
		}),
	)
	return err
}

func upsertFeedInfo(ctx context.Context, tx pgx.Tx, info models.FeedInfo) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trains.feed_info
			(singleton, feed_version, feed_start_date, feed_end_date,
			 feed_lang, etag, last_modified, parser_version,
			 translated_stops_nl, translated_stops_fr, translated_stops_en,
			 translation_rows, translation_rows_unmatched, imported_at)
		VALUES (TRUE, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (singleton) DO UPDATE SET
			feed_version    = EXCLUDED.feed_version,
			feed_start_date = EXCLUDED.feed_start_date,
			feed_end_date   = EXCLUDED.feed_end_date,
			feed_lang       = EXCLUDED.feed_lang,
			etag            = EXCLUDED.etag,
			last_modified   = EXCLUDED.last_modified,
			parser_version  = EXCLUDED.parser_version,
			translated_stops_nl = EXCLUDED.translated_stops_nl,
			translated_stops_fr = EXCLUDED.translated_stops_fr,
			translated_stops_en = EXCLUDED.translated_stops_en,
			translation_rows = EXCLUDED.translation_rows,
			translation_rows_unmatched = EXCLUDED.translation_rows_unmatched,
			imported_at     = now()
	`,
		info.FeedVersion, info.StartDate, info.EndDate, nullStr(info.Lang),
		nullStr(info.ETag), nullStr(info.LastModified), info.ParserVersion,
		info.Translations.StopsNL, info.Translations.StopsFR,
		info.Translations.StopsEN, info.Translations.Rows,
		info.Translations.RowsUnmatched,
	)
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
