package irail

import (
	"strconv"
	"strings"
	"time"
)

// brussels is the timezone iRail's date and time parameters are read in.
//
//nolint:gochecknoglobals //fixed timezone
var brussels = loadBrussels()

func loadBrussels() *time.Location {
	loc, err := time.LoadLocation("Europe/Brussels")
	if err != nil {
		return time.UTC
	}
	return loc
}

// stationPrefix precedes the UIC code in iRail station ids.
const stationPrefix = "BE.NMBS."

// Station is a station with its name in the requested language.
type Station struct {
	ID   string
	Name string
}

// Endpoint is where a leg boards or alights. Time is the scheduled time;
// Delay is live, in seconds (positive = late).
type Endpoint struct {
	StationID string
	Name      string
	Time      time.Time
	Delay     int
	Platform  string
	Canceled  bool
}

// Leg is one boarded train, board to alight.
type Leg struct {
	// VehicleID is iRail's id, e.g. "BE.NMBS.IC530".
	VehicleID string
	// Type and Number split the vehicle's short name ("IC", "530").
	Type      string
	Number    string
	Direction string
	Board     Endpoint
	Alight    Endpoint
}

// Alert is a disruption notice attached to a connection.
type Alert struct {
	Header      string
	Description string
}

// Connection is one planned journey.
type Connection struct {
	Legs   []Leg
	Alerts []Alert
}

// Call is one stop of a vehicle's run.
type Call struct {
	StationID          string
	Name               string
	Platform           string
	ScheduledArrival   time.Time
	ScheduledDeparture time.Time
	ArrivalDelay       int
	DepartureDelay     int
	ArrivalCanceled    bool
	DepartureCanceled  bool
}

// Vehicle is one train's full run with live state.
type Vehicle struct {
	ID    string
	Type  string
	Num   string
	Calls []Call
}

// Canceled reports a whole-trip cancellation: every stop is cancelled.
func (v *Vehicle) Canceled() bool {
	if len(v.Calls) == 0 {
		return false
	}
	for _, c := range v.Calls {
		if !c.ArrivalCanceled && !c.DepartureCanceled {
			return false
		}
	}
	return true
}

// StationUIC strips iRail's "BE.NMBS." prefix: "BE.NMBS.008812005" becomes
// "008812005", the id the app uses everywhere.
func StationUIC(id string) string {
	return strings.TrimPrefix(id, stationPrefix)
}

// ShortName strips the "BE.NMBS." prefix from a vehicle id: "IC530".
func ShortName(vehicleID string) string {
	return strings.TrimPrefix(vehicleID, stationPrefix)
}

// VehicleID rebuilds iRail's vehicle id from a short name like "IC530".
func VehicleID(shortName string) string {
	return stationPrefix + strings.ReplaceAll(shortName, " ", "")
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func unixTime(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}
