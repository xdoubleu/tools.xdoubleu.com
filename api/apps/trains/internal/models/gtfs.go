// Package models holds the domain types for the trains app's ingested GTFS
// static feed.
package models

import "time"

// Stop is one row of stops.txt. In this feed a station has
// location_type=1 and stop_id "gs:nmbssncb:S<uic>"; each platform has
// location_type=0, its own "gs:nmbssncb:<uic>[_<n>]" id and parent_station
// pointing at the station (issue #1389).
type Stop struct {
	StopID        string
	ParentStation string
	// NameNL/NameFR/NameEN are the stop name in each language. The feed's
	// stop_name column carries a single (primary) language; translations.txt,
	// when the feed publishes it, supplies the other two. A language with no
	// translation falls back to the primary stop_name (issue #1450).
	NameNL       string
	NameFR       string
	NameEN       string
	LocationType int
	PlatformCode string
	// UIC is the bare 7-digit code parsed out of StopID.
	UIC string
	Lat *float64
	Lon *float64
}

// Route is one row of routes.txt.
type Route struct {
	RouteID   string
	ShortName string
	LongName  string
	RouteType int
}

// Trip is one row of trips.txt. TripID is a seasonal stopping-pattern
// variant that churns daily — ShortName is the stable published train
// number (issue #1388).
type Trip struct {
	TripID      string
	RouteID     string
	ServiceID   string
	ShortName   string
	Headsign    string
	DirectionID *int
}

// StopTime is one row of stop_times.txt. Arrival/Departure are seconds since
// GTFS midnight and may legitimately exceed 86400.
type StopTime struct {
	TripID           string
	StopSequence     int
	StopID           string
	ArrivalSeconds   int
	DepartureSeconds int
	PickupType       int
	DropOffType      int
}

// CalendarDate is one row of calendar_dates.txt. In this feed every row is
// exception_type=1 (added) and calendar.txt itself is a decoy (issue #1390).
type CalendarDate struct {
	ServiceID     string
	Date          time.Time
	ExceptionType int
}

// FeedInfo is the single row of feed_info.txt, plus the conditional-GET
// validators from the fetch that produced this import.
type FeedInfo struct {
	FeedVersion  string
	StartDate    *time.Time
	EndDate      *time.Time
	Lang         string
	ETag         string
	LastModified string
	// ImportedAt is when the import that produced these rows ran. Set by the
	// database on write, so it is only populated on a read.
	ImportedAt *time.Time
	// ParserVersion identifies the importer that produced the stored rows.
	// The import compares it against the current version to decide whether
	// the conditional-GET validators above still describe usable data
	// (issue #1453).
	ParserVersion int
	// Translations records how much of translations.txt the import actually
	// applied (issue #1459).
	Translations TranslationCoverage
}

// TranslationCoverage is what one import made of translations.txt. A feed
// that publishes no translations, one whose rows this importer cannot match
// to a stop, and one that is simply monolingual all produce the same
// user-visible result — three identical station names — so the counts are
// stored and served alongside the feed to tell those cases apart
// (issue #1459).
type TranslationCoverage struct {
	// StopsNL/StopsFR/StopsEN count the stops whose name in that language
	// came from translations.txt rather than falling back to the primary
	// stop_name. A count of 0 against a non-zero Rows means the rows were
	// read but matched nothing.
	StopsNL int
	StopsFR int
	StopsEN int
	// Rows is the number of usable stop_name rows read from translations.txt
	// — 0 when the feed omits the file entirely.
	Rows int
	// RowsUnmatched counts the (key, language) pairs among those Rows that
	// identified a stop this feed's stops.txt does not contain. Duplicate
	// rows for one pair collapse, so against a feed that repeats itself this
	// reads slightly below Rows even when nothing matched.
	RowsUnmatched int
}

// Transfer is one row of transfers.txt. TransferType follows the GTFS
// enum (0/1 = recommended/timed, min_transfer_time only meaningful for
// type 2, 3 = not possible). Present only when the feed publishes it — the
// router falls back to a default minimum transfer time otherwise (issue
// #1391).
type Transfer struct {
	FromStopID      string
	ToStopID        string
	TransferType    int
	MinTransferTime *int
}

// Feed is a fully parsed static feed, ready to be swapped into the trains
// schema in one transaction.
type Feed struct {
	Info          FeedInfo
	Stops         []Stop
	Routes        []Route
	Trips         []Trip
	StopTimes     []StopTime
	CalendarDates []CalendarDate
	Transfers     []Transfer
}
