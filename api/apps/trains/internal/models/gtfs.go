// Package models holds the trains app's domain types.
package models

import "time"

// Stop is one row of stops.txt. Stations have location_type=1 and stop_id
// "gs:nmbssncb:S<uic>"; platforms have location_type=0, "gs:nmbssncb:<uic>[_<n>]"
// and parent_station set.
type Stop struct {
	StopID        string
	ParentStation string
	// Per-language names; a language without a translation falls back to the
	// primary stop_name.
	NameNL string
	NameFR string
	NameEN string
	// DisplayName is every genuinely-known name (primary plus actual
	// translations), deduped and " / "-joined. It never uses the fallback, since
	// the raw stop_name is sometimes already an abbreviated bilingual string.
	DisplayName  string
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

// Trip is one row of trips.txt. TripID churns daily; ShortName is the stable
// train number.
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

// ActiveTrip is one (trip, service day) resolved from calendar_dates alone;
// calendar.txt is a decoy in this feed.
type ActiveTrip struct {
	TripID         string
	RouteID        string
	TripShortName  string
	RouteShortName string
	TripHeadsign   string
	Date           time.Time
}

// CalendarDate is one row of calendar_dates.txt (always exception_type=1).
type CalendarDate struct {
	ServiceID     string
	Date          time.Time
	ExceptionType int
}

// FeedInfo is feed_info.txt plus the conditional-GET validators of its fetch.
type FeedInfo struct {
	FeedVersion  string
	StartDate    *time.Time
	EndDate      *time.Time
	Lang         string
	ETag         string
	LastModified string
	// ImportedAt is set by the database, so only populated on read.
	ImportedAt *time.Time
	// ParserVersion identifies the importer that wrote the rows; a mismatch
	// invalidates the stored validators.
	ParserVersion int
	// Translations records how much of translations.txt the import applied.
	Translations TranslationCoverage
}

// TranslationCoverage distinguishes "no translations", "unmatched rows" and
// "monolingual feed", which all look identical to a user.
type TranslationCoverage struct {
	// StopsNL/StopsFR/StopsEN count stops named from translations.txt; 0 with
	// non-zero Rows means nothing matched.
	StopsNL int
	StopsFR int
	StopsEN int
	// Rows is the usable stop_name row count; 0 when the file is absent.
	Rows int
	// RowsUnmatched counts distinct (key, language) pairs matching no stop.
	RowsUnmatched int
}

// Transfer is one row of transfers.txt (GTFS transfer_type enum). The router
// uses a default minimum transfer time when the feed has none.
type Transfer struct {
	FromStopID      string
	ToStopID        string
	TransferType    int
	MinTransferTime *int
}

// Feed is a fully parsed static feed.
type Feed struct {
	Info          FeedInfo
	Stops         []Stop
	Routes        []Route
	Trips         []Trip
	StopTimes     []StopTime
	CalendarDates []CalendarDate
	Transfers     []Transfer
}
