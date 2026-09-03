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
	Name          string
	LocationType  int
	PlatformCode  string
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
}
