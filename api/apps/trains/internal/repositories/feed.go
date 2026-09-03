package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	var etag, lastModified *string
	err := r.db.QueryRow(ctx, `
		SELECT feed_version, feed_start_date, feed_end_date, feed_lang,
		       etag, last_modified
		FROM trains.feed_info
		WHERE singleton
	`).Scan(
		&info.FeedVersion, &info.StartDate, &info.EndDate, &info.Lang,
		&etag, &lastModified,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil //"nothing imported yet" is a valid state
	}
	if err != nil {
		return nil, err
	}
	if etag != nil {
		info.ETag = *etag
	}
	if lastModified != nil {
		info.LastModified = *lastModified
	}
	return info, nil
}

// staged is the ordered set of tables ImportFeed replaces. TRUNCATE + COPY
// inside a single transaction is an atomic swap for readers under MVCC: a
// router querying the schema sees the complete old feed until COMMIT, then
// the complete new one — never a half-replaced mix (issue #1390).
//
//nolint:gochecknoglobals //fixed table list, package-level by design
var stagedTables = []string{
	"trains.stop_times",
	"trains.calendar_dates",
	"trains.trips",
	"trains.routes",
	"trains.stops",
}

// ImportFeed replaces the entire trains timetable with feed in one
// transaction.
func (r *FeedRepository) ImportFeed(
	ctx context.Context,
	feed *models.Feed,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(
		ctx, fmt.Sprintf("TRUNCATE %s", strings.Join(stagedTables, ", ")),
	); err != nil {
		return err
	}

	if err = copyStops(ctx, tx, feed.Stops); err != nil {
		return err
	}
	if err = copyRoutes(ctx, tx, feed.Routes); err != nil {
		return err
	}
	if err = copyTrips(ctx, tx, feed.Trips); err != nil {
		return err
	}
	if err = copyStopTimes(ctx, tx, feed.StopTimes); err != nil {
		return err
	}
	if err = copyCalendarDates(ctx, tx, feed.CalendarDates); err != nil {
		return err
	}
	if err = upsertFeedInfo(ctx, tx, feed.Info); err != nil {
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
		pgx.Identifier{schemaName, "stops"},
		[]string{
			"stop_id", "parent_station", "name", "location_type",
			"platform_code", "uic", "lat", "lon",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			s := rows[i]
			return []any{
				s.StopID, nullStr(s.ParentStation), s.Name, s.LocationType,
				nullStr(s.PlatformCode), nullStr(s.UIC), s.Lat, s.Lon,
			}, nil
		}),
	)
	return err
}

func copyRoutes(ctx context.Context, tx pgx.Tx, rows []models.Route) error {
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{schemaName, "routes"},
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
		pgx.Identifier{schemaName, "trips"},
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
		pgx.Identifier{schemaName, "stop_times"},
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
		pgx.Identifier{schemaName, "calendar_dates"},
		[]string{"service_id", "date", "exception_type"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			cd := rows[i]
			return []any{cd.ServiceID, cd.Date, cd.ExceptionType}, nil
		}),
	)
	return err
}

func upsertFeedInfo(ctx context.Context, tx pgx.Tx, info models.FeedInfo) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trains.feed_info
			(singleton, feed_version, feed_start_date, feed_end_date,
			 feed_lang, etag, last_modified, imported_at)
		VALUES (TRUE, $1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (singleton) DO UPDATE SET
			feed_version    = EXCLUDED.feed_version,
			feed_start_date = EXCLUDED.feed_start_date,
			feed_end_date   = EXCLUDED.feed_end_date,
			feed_lang       = EXCLUDED.feed_lang,
			etag            = EXCLUDED.etag,
			last_modified   = EXCLUDED.last_modified,
			imported_at     = now()
	`,
		info.FeedVersion, info.StartDate, info.EndDate, nullStr(info.Lang),
		nullStr(info.ETag), nullStr(info.LastModified),
	)
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
