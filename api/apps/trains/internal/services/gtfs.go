package services

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// zipMagic is the local-file-header signature every real zip starts with.
// The feed must be verified by these bytes, not by Content-Type: at least
// one Belgian GTFS mirror answers 200 + "application/zip" with an HTML
// domain-squat body (issue #1389).
//
//nolint:gochecknoglobals //package-level constant byte slice
var zipMagic = []byte{'P', 'K', 0x03, 0x04}

// maxStopTimeSeconds bounds a stop_times value. GTFS legitimately allows
// times past 24:00:00 for after-midnight service, but values up to 87:39:00
// (3.6 days) were observed and are a publisher bug — rows beyond this are
// rejected and counted rather than generating phantom connections (#1390).
const maxStopTimeSeconds = 36 * 3600

var errZipMagic = errors.New("trains: download is not a zip (bad magic bytes)")

// gtfsPrefix is stripped from every stop_id to recover the bare UIC code.
const gtfsPrefix = "gs:nmbssncb:"

// parseFeed parses a GTFS static zip into a Feed. Rejected stop_times rows
// are logged with a count; everything else is either parsed or fails the
// whole import.
func parseFeed(logger *slog.Logger, raw []byte) (*models.Feed, error) {
	if len(raw) < len(zipMagic) || !bytes.Equal(raw[:len(zipMagic)], zipMagic) {
		return nil, errZipMagic
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("trains: opening zip: %w", err)
	}

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	//nolint:exhaustruct //filled field by field below
	feed := &models.Feed{}

	if feed.Info, err = parseFeedInfo(files); err != nil {
		return nil, err
	}
	if feed.Stops, err = parseStops(files); err != nil {
		return nil, err
	}
	if feed.Routes, err = parseRoutes(files); err != nil {
		return nil, err
	}
	if feed.Trips, err = parseTrips(files); err != nil {
		return nil, err
	}
	if feed.StopTimes, err = parseStopTimes(logger, files); err != nil {
		return nil, err
	}
	if feed.CalendarDates, err = parseCalendarDates(files); err != nil {
		return nil, err
	}
	if feed.Transfers, err = parseTransfers(files); err != nil {
		return nil, err
	}

	return feed, nil
}

// rowReader iterates a GTFS csv file, resolving columns by header name —
// this feed orders columns alphabetically, so positional parsing would
// break silently (issue #1389).
type rowReader struct {
	r   *csv.Reader
	col map[string]int
	rc  io.ReadCloser
}

func openRows(files map[string]*zip.File, name string) (*rowReader, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("trains: %s missing from feed", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	cr := csv.NewReader(rc)
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("trains: %s header: %w", name, err)
	}
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")] = i
	}
	return &rowReader{r: cr, col: col, rc: rc}, nil
}

func (rr *rowReader) next() ([]string, error) { return rr.r.Read() }
func (rr *rowReader) close()                  { _ = rr.rc.Close() }

func (rr *rowReader) get(rec []string, name string) string {
	i, ok := rr.col[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func (rr *rowReader) getInt(rec []string, name string) int {
	n, _ := strconv.Atoi(rr.get(rec, name))
	return n
}

func parseStops(files map[string]*zip.File) ([]models.Stop, error) {
	rr, openErr := openRows(files, "stops.txt")
	if openErr != nil {
		return nil, openErr
	}
	defer rr.close()

	var out []models.Stop
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		id := rr.get(rec, "stop_id")
		if id == "" {
			continue
		}
		out = append(out, models.Stop{
			StopID:        id,
			ParentStation: rr.get(rec, "parent_station"),
			Name:          rr.get(rec, "stop_name"),
			LocationType:  rr.getInt(rec, "location_type"),
			PlatformCode:  rr.get(rec, "platform_code"),
			UIC:           uicFromStopID(id),
			Lat:           parseFloatPtr(rr.get(rec, "stop_lat")),
			Lon:           parseFloatPtr(rr.get(rec, "stop_lon")),
		})
	}
	return out, nil
}

func parseRoutes(files map[string]*zip.File) ([]models.Route, error) {
	rr, openErr := openRows(files, "routes.txt")
	if openErr != nil {
		return nil, openErr
	}
	defer rr.close()

	var out []models.Route
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		id := rr.get(rec, "route_id")
		if id == "" {
			continue
		}
		out = append(out, models.Route{
			RouteID:   id,
			ShortName: rr.get(rec, "route_short_name"),
			LongName:  rr.get(rec, "route_long_name"),
			RouteType: rr.getInt(rec, "route_type"),
		})
	}
	return out, nil
}

func parseTrips(files map[string]*zip.File) ([]models.Trip, error) {
	rr, openErr := openRows(files, "trips.txt")
	if openErr != nil {
		return nil, openErr
	}
	defer rr.close()

	var out []models.Trip
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		id := rr.get(rec, "trip_id")
		if id == "" {
			continue
		}
		out = append(out, models.Trip{
			TripID:      id,
			RouteID:     rr.get(rec, "route_id"),
			ServiceID:   rr.get(rec, "service_id"),
			ShortName:   rr.get(rec, "trip_short_name"),
			Headsign:    rr.get(rec, "trip_headsign"),
			DirectionID: parseIntPtr(rr.get(rec, "direction_id")),
		})
	}
	return out, nil
}

func parseStopTimes(
	logger *slog.Logger, files map[string]*zip.File,
) ([]models.StopTime, error) {
	rr, openErr := openRows(files, "stop_times.txt")
	if openErr != nil {
		return nil, openErr
	}
	defer rr.close()

	var out []models.StopTime
	var rejected int
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		tripID := rr.get(rec, "trip_id")
		if tripID == "" {
			continue
		}
		arr, arrOK := parseGTFSTime(rr.get(rec, "arrival_time"))
		dep, depOK := parseGTFSTime(rr.get(rec, "departure_time"))
		if !arrOK || !depOK {
			rejected++
			continue
		}
		out = append(out, models.StopTime{
			TripID:           tripID,
			StopSequence:     rr.getInt(rec, "stop_sequence"),
			StopID:           rr.get(rec, "stop_id"),
			ArrivalSeconds:   arr,
			DepartureSeconds: dep,
			PickupType:       rr.getInt(rec, "pickup_type"),
			DropOffType:      rr.getInt(rec, "drop_off_type"),
		})
	}
	if rejected > 0 {
		logger.Warn("trains: rejected out-of-bounds stop_times rows",
			slog.Int("rejected", rejected),
			slog.Int("bound_seconds", maxStopTimeSeconds),
		)
	}
	return out, nil
}

func parseCalendarDates(
	files map[string]*zip.File,
) ([]models.CalendarDate, error) {
	rr, openErr := openRows(files, "calendar_dates.txt")
	if openErr != nil {
		return nil, openErr
	}
	defer rr.close()

	var out []models.CalendarDate
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		svc := rr.get(rec, "service_id")
		date, ok := parseGTFSDate(rr.get(rec, "date"))
		if svc == "" || !ok {
			continue
		}
		out = append(out, models.CalendarDate{
			ServiceID:     svc,
			Date:          date,
			ExceptionType: rr.getInt(rec, "exception_type"),
		})
	}
	return out, nil
}

// parseTransfers parses transfers.txt. Unlike every other GTFS file this
// one is optional in the feed — a missing file yields no rows rather than
// an error, and the router falls back to a default minimum transfer time
// at same-station changes (issue #1391).
func parseTransfers(files map[string]*zip.File) ([]models.Transfer, error) {
	rr, openErr := openRows(files, "transfers.txt")
	if openErr != nil {
		return nil, nil
	}
	defer rr.close()

	var out []models.Transfer
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		from := rr.get(rec, "from_stop_id")
		to := rr.get(rec, "to_stop_id")
		if from == "" || to == "" {
			continue
		}
		out = append(out, models.Transfer{
			FromStopID:      from,
			ToStopID:        to,
			TransferType:    rr.getInt(rec, "transfer_type"),
			MinTransferTime: parseIntPtr(rr.get(rec, "min_transfer_time")),
		})
	}
	return out, nil
}

func parseFeedInfo(files map[string]*zip.File) (models.FeedInfo, error) {
	//nolint:exhaustruct //validators set by the caller
	info := models.FeedInfo{}
	rr, openErr := openRows(files, "feed_info.txt")
	if openErr != nil {
		return info, openErr
	}
	defer rr.close()

	rec, err := rr.next()
	if err != nil {
		return info, fmt.Errorf("trains: feed_info row: %w", err)
	}
	info.FeedVersion = rr.get(rec, "feed_version")
	info.Lang = rr.get(rec, "feed_lang")
	if d, ok := parseGTFSDate(rr.get(rec, "feed_start_date")); ok {
		info.StartDate = &d
	}
	if d, ok := parseGTFSDate(rr.get(rec, "feed_end_date")); ok {
		info.EndDate = &d
	}
	if info.FeedVersion == "" {
		return info, errors.New("trains: feed_info has no feed_version")
	}
	return info, nil
}

// parseGTFSTime parses "H:MM:SS" (H may be >= 24, multi-digit) into seconds
// since GTFS midnight. Returns ok=false for an unparseable value or one
// beyond maxStopTimeSeconds.
func parseGTFSTime(v string) (int, bool) {
	parts := strings.Split(v, ":")
	const hmsParts = 3
	if len(parts) != hmsParts {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	s, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	if h < 0 || m < 0 || m > 59 || s < 0 || s > 59 {
		return 0, false
	}
	total := h*3600 + m*60 + s
	if total > maxStopTimeSeconds {
		return 0, false
	}
	return total, true
}

func parseGTFSDate(v string) (time.Time, bool) {
	t, err := time.Parse("20060102", v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// uicFromStopID recovers the bare 7-digit UIC from a feed stop_id such as
// "gs:nmbssncb:S8814001" or "gs:nmbssncb:8814001_3".
func uicFromStopID(id string) string {
	s := strings.TrimPrefix(id, gtfsPrefix)
	s = strings.TrimPrefix(s, "S")
	if i := strings.IndexByte(s, '_'); i >= 0 {
		s = s[:i]
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}

func parseFloatPtr(v string) *float64 {
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseIntPtr(v string) *int {
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}
