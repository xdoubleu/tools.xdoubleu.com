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

// zipMagic is the zip local-file-header signature. Verify by bytes, not
// Content-Type: some mirrors serve HTML with "application/zip".
//
//nolint:gochecknoglobals //package-level constant byte slice
var zipMagic = []byte{'P', 'K', 0x03, 0x04}

// maxStopTimeSeconds bounds stop_times values. Values past 24:00 are valid,
// but multi-day ones are publisher bugs and are rejected.
const maxStopTimeSeconds = 36 * 3600

var errZipMagic = errors.New("trains: download is not a zip (bad magic bytes)")

const gtfsPrefix = "gs:nmbssncb:"

// parseFeed parses a GTFS static zip. Bad stop_times rows are counted and
// skipped; any other error fails the import.
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
	translations, err := parseTranslations(files)
	if err != nil {
		return nil, err
	}
	if feed.Stops, feed.Info.Translations, err = parseStops(
		files, translations, feed.Info.Lang,
	); err != nil {
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

// rowReader resolves columns by header name; this feed orders them
// alphabetically.
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

// parseStops parses stops.txt; stop_name is in primaryLang and translations
// fill the other languages.
func parseStops(
	files map[string]*zip.File,
	translations *stopTranslations,
	primaryLang string,
) ([]models.Stop, models.TranslationCoverage, error) {
	//nolint:exhaustruct //counted up below
	coverage := models.TranslationCoverage{Rows: translations.rows}

	rr, openErr := openRows(files, "stops.txt")
	if openErr != nil {
		return nil, coverage, openErr
	}
	defer rr.close()

	var out []models.Stop
	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, coverage, err
		}
		id := rr.get(rec, "stop_id")
		if id == "" {
			continue
		}
		name := rr.get(rec, "stop_name")
		translated := translations.forStop(id, name)
		names := map[string]string{normalizeLang(primaryLang): name}
		for lang, t := range translated {
			names[lang] = t
		}
		nameOrFallback := func(lang string) string {
			if n, ok := names[lang]; ok {
				return n
			}
			return name
		}
		displayName := buildDisplayName(primaryLang, name, translated)
		if _, ok := translated["nl"]; ok {
			coverage.StopsNL++
		}
		if _, ok := translated["fr"]; ok {
			coverage.StopsFR++
		}
		if _, ok := translated["en"]; ok {
			coverage.StopsEN++
		}
		out = append(out, models.Stop{
			StopID:        id,
			ParentStation: rr.get(rec, "parent_station"),
			NameNL:        nameOrFallback("nl"),
			NameFR:        nameOrFallback("fr"),
			NameEN:        nameOrFallback("en"),
			DisplayName:   displayName,
			LocationType:  rr.getInt(rec, "location_type"),
			PlatformCode:  rr.get(rec, "platform_code"),
			UIC:           uicFromStopID(id),
			Lat:           parseFloatPtr(rr.get(rec, "stop_lat")),
			Lon:           parseFloatPtr(rr.get(rec, "stop_lon")),
		})
	}

	coverage.RowsUnmatched = translations.unmatchedRows()
	return out, coverage, nil
}

// stopTranslations holds translations.txt stop_name rows by both allowed keys.
type stopTranslations struct {
	// byRecordID/byValue map a record_id / field_value to language -> name.
	byRecordID map[string]map[string]string
	byValue    map[string]map[string]string
	// used records matched keys as "<index>\x00<key>".
	used map[string]bool
	rows int
}

// forStop returns one stop's translations by language and marks rows used.
// Tries, in order: full stop_id, stop_id without gtfsPrefix (the gateway
// prefixes stops.txt but not translations.txt), then stop_name (field_value).
func (t *stopTranslations) forStop(stopID, name string) map[string]string {
	candidates := []struct {
		index string
		key   string
		from  map[string]map[string]string
	}{
		{"id", stopID, t.byRecordID},
		{"id", strings.TrimPrefix(stopID, gtfsPrefix), t.byRecordID},
		{"value", name, t.byValue},
	}

	out := map[string]string{}
	for _, c := range candidates {
		if c.key == "" {
			continue
		}
		names, ok := c.from[c.key]
		if !ok {
			continue
		}
		t.used[c.index+"\x00"+c.key] = true
		// An earlier, more specific candidate wins per language.
		for lang, translated := range names {
			if _, taken := out[lang]; !taken {
				out[lang] = translated
			}
		}
	}
	return out
}

func (t *stopTranslations) unmatchedRows() int {
	unmatched := 0
	for index, from := range map[string]map[string]map[string]string{
		"id":    t.byRecordID,
		"value": t.byValue,
	} {
		for key, names := range from {
			if !t.used[index+"\x00"+key] {
				unmatched += len(names)
			}
		}
	}
	return unmatched
}

// parseTranslations indexes the optional translations.txt stop_name rows;
// a missing file yields an empty index.
func parseTranslations(files map[string]*zip.File) (*stopTranslations, error) {
	out := &stopTranslations{
		byRecordID: map[string]map[string]string{},
		byValue:    map[string]map[string]string{},
		used:       map[string]bool{},
		rows:       0,
	}

	rr, openErr := openRows(files, "translations.txt")
	if openErr != nil {
		return out, nil
	}
	defer rr.close()

	for {
		rec, err := rr.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if rr.get(rec, "table_name") != "stops" ||
			rr.get(rec, "field_name") != "stop_name" {
			continue
		}
		lang := normalizeLang(rr.get(rec, "language"))
		translation := rr.get(rec, "translation")
		if lang == "" || translation == "" {
			continue
		}
		// record_id is more specific; prefer it if a publisher sets both.
		index, key := out.byValue, rr.get(rec, "field_value")
		if recordID := rr.get(rec, "record_id"); recordID != "" {
			index, key = out.byRecordID, recordID
		}
		if key == "" {
			continue
		}
		if index[key] == nil {
			index[key] = map[string]string{}
		}
		index[key][lang] = translation
		out.rows++
	}
	return out, nil
}

// normalizeLang maps a GTFS language tag to a two-letter code, or "".
func normalizeLang(v string) string {
	v = strings.ToLower(v)
	switch {
	case strings.HasPrefix(v, "nl"):
		return "nl"
	case strings.HasPrefix(v, "fr"):
		return "fr"
	case strings.HasPrefix(v, "en"):
		return "en"
	default:
		return ""
	}
}

// displayNameLangs is the language order buildDisplayName uses.
//
//nolint:gochecknoglobals //fixed language list, package-level by design
var displayNameLangs = [3]string{"nl", "fr", "en"}

// isSyntheticCombined reports whether a translation is an NMBS-synthesized
// multi-language string (e.g. "Roeselare / Roulers"), not a real name. Real
// Belgian station names never contain "/".
func isSyntheticCombined(v string) bool {
	return strings.Contains(v, "/")
}

// buildDisplayName " / "-joins each language's genuine full name: a
// non-synthetic translation, else the primary stop_name for the primary
// language only. Languages without a usable translation are omitted, since
// the raw stop_name is sometimes itself an abbreviated bilingual string.
func buildDisplayName(primaryLang, name string, translated map[string]string) string {
	primary := normalizeLang(primaryLang)
	seen := make(map[string]bool, len(displayNameLangs))
	parts := make([]string, 0, len(displayNameLangs))
	add := func(n string) {
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		parts = append(parts, n)
	}
	if t, ok := translated[primary]; ok && !isSyntheticCombined(t) {
		add(t)
	} else {
		add(name)
	}
	for _, lang := range displayNameLangs {
		if lang == primary {
			continue
		}
		if t, ok := translated[lang]; ok && !isSyntheticCombined(t) {
			add(t)
		}
	}
	return strings.Join(parts, " / ")
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

// parseTransfers parses the optional transfers.txt; a missing file yields no
// rows and the router uses a default minimum transfer time.
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

// parseGTFSTime parses "H:MM:SS" (H may be >= 24) into seconds; ok=false if
// unparseable or beyond maxStopTimeSeconds.
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

// uicFromStopID recovers the 7-digit UIC from e.g. "gs:nmbssncb:S8814001".
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
