package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// ActiveTrip is one (trip, service day) combination resolved from
// calendar_dates alone — never calendar.txt, which is a decoy in this feed
// (issue #1390) — for a trip whose service runs on Date within the
// router's rolling window.
type ActiveTrip struct {
	TripID         string
	RouteID        string
	TripShortName  string
	RouteShortName string
	TripHeadsign   string
	Date           time.Time
}

// AllStops returns every stop (652 in this feed — cheap to load whole).
func (r *FeedRepository) AllStops(ctx context.Context) ([]models.Stop, error) {
	rows, err := r.db.Query(ctx, `
		SELECT stop_id, parent_station, name_nl, name_fr, name_en,
		       location_type, platform_code, uic
		FROM trains.stops
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Stop
	for rows.Next() {
		var s models.Stop
		var parent, platform, uic *string
		if err = rows.Scan(
			&s.StopID, &parent, &s.NameNL, &s.NameFR, &s.NameEN,
			&s.LocationType, &platform, &uic,
		); err != nil {
			return nil, err
		}
		if parent != nil {
			s.ParentStation = *parent
		}
		if platform != nil {
			s.PlatformCode = *platform
		}
		if uic != nil {
			s.UIC = *uic
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// AllTransfers returns every transfers.txt row (small — station-pair
// footpaths, not per stop_time).
func (r *FeedRepository) AllTransfers(ctx context.Context) ([]models.Transfer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_stop_id, to_stop_id, transfer_type, min_transfer_time
		FROM trains.transfers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Transfer
	for rows.Next() {
		var t models.Transfer
		if err = rows.Scan(
			&t.FromStopID, &t.ToStopID, &t.TransferType, &t.MinTransferTime,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ActiveTripsInWindow resolves, from calendar_dates only, every trip whose
// service runs on some date in [start, end] (inclusive) — the router's
// rolling window (issue #1391). It returns one row per (trip, date).
func (r *FeedRepository) ActiveTripsInWindow(
	ctx context.Context, start, end time.Time,
) ([]ActiveTrip, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.trip_id, t.route_id, t.trip_short_name, r.short_name,
		       t.trip_headsign, cd.date
		FROM trains.calendar_dates cd
		JOIN trains.trips t ON t.service_id = cd.service_id
		JOIN trains.routes r ON r.route_id = t.route_id
		WHERE cd.exception_type = 1 AND cd.date BETWEEN $1 AND $2
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ActiveTrip
	for rows.Next() {
		var at ActiveTrip
		var shortName, routeShortName, headsign *string
		if err = rows.Scan(
			&at.TripID, &at.RouteID, &shortName, &routeShortName,
			&headsign, &at.Date,
		); err != nil {
			return nil, err
		}
		if shortName != nil {
			at.TripShortName = *shortName
		}
		if routeShortName != nil {
			at.RouteShortName = *routeShortName
		}
		if headsign != nil {
			at.TripHeadsign = *headsign
		}
		out = append(out, at)
	}
	return out, rows.Err()
}

// TripByShortNameOnDate resolves the trip_id running trip_short_name on
// date, the same way ActiveTripsInWindow resolves a whole window — used to
// map a live journey's boarded trip back to its full stop_times pattern for
// the day it actually ran (issue #1394). Returns (nil, nil) when no active
// trip matches (the timetable changed, or an old journey page is still open
// past the router's rolling window).
func (r *FeedRepository) TripByShortNameOnDate(
	ctx context.Context, tripShortName string, date time.Time,
) (*ActiveTrip, error) {
	var at ActiveTrip
	var shortName, routeShortName, headsign *string
	err := r.db.QueryRow(ctx, `
		SELECT t.trip_id, t.route_id, t.trip_short_name, r.short_name,
		       t.trip_headsign, cd.date
		FROM trains.calendar_dates cd
		JOIN trains.trips t ON t.service_id = cd.service_id
		JOIN trains.routes r ON r.route_id = t.route_id
		WHERE cd.exception_type = 1 AND cd.date = $1 AND t.trip_short_name = $2
		LIMIT 1
	`, date, tripShortName).Scan(
		&at.TripID, &at.RouteID, &shortName, &routeShortName, &headsign, &at.Date,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil //"no active trip on this date" is a valid state
	}
	if err != nil {
		return nil, err
	}
	if shortName != nil {
		at.TripShortName = *shortName
	}
	if routeShortName != nil {
		at.RouteShortName = *routeShortName
	}
	if headsign != nil {
		at.TripHeadsign = *headsign
	}
	return &at, nil
}

// StopsByIDs returns the stops matching ids — used to attach names/platforms
// to a journey detail's stop calls without loading the full stops table
// (issue #1394).
func (r *FeedRepository) StopsByIDs(
	ctx context.Context, ids []string,
) ([]models.Stop, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT stop_id, parent_station, name_nl, name_fr, name_en,
		       location_type, platform_code, uic
		FROM trains.stops
		WHERE stop_id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Stop
	for rows.Next() {
		var s models.Stop
		var parent, platform, uic *string
		if err = rows.Scan(
			&s.StopID, &parent, &s.NameNL, &s.NameFR, &s.NameEN,
			&s.LocationType, &platform, &uic,
		); err != nil {
			return nil, err
		}
		if parent != nil {
			s.ParentStation = *parent
		}
		if platform != nil {
			s.PlatformCode = *platform
		}
		if uic != nil {
			s.UIC = *uic
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// StopTimesForTrips returns every stop_times row for tripIDs, ordered by
// trip then stop_sequence — the caller (the router builder) batches this
// over the trip IDs resolved by ActiveTripsInWindow rather than ever
// scanning the full ~0.8M-row table (convention-database-queries.md).
func (r *FeedRepository) StopTimesForTrips(
	ctx context.Context, tripIDs []string,
) ([]models.StopTime, error) {
	if len(tripIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT trip_id, stop_sequence, stop_id, arrival_seconds,
		       departure_seconds, pickup_type, drop_off_type
		FROM trains.stop_times
		WHERE trip_id = ANY($1)
		ORDER BY trip_id, stop_sequence
	`, tripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StopTime
	for rows.Next() {
		var st models.StopTime
		if err = rows.Scan(
			&st.TripID, &st.StopSequence, &st.StopID, &st.ArrivalSeconds,
			&st.DepartureSeconds, &st.PickupType, &st.DropOffType,
		); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
