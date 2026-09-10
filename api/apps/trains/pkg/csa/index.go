// Package csa builds an in-memory Connection Scan Algorithm index over a
// rolling window of the trains schema's ingested GTFS timetable, and
// answers journey searches over it. It never touches trip_id in anything
// it returns to a caller outside this package — trip identity is
// per-instance and used only to group elementary connections back into a
// leg while scanning (issue #1391, following the trip_id-churn invariant
// from #1390).
package csa

import (
	"sort"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// stopDisplayName is the station name the planner puts on a Leg. The feed
// carries three (models.Stop.NameNL/FR/EN); SearchJourneys' legs are
// single-language and use French, the feed's own primary language — the
// realtime journey-detail page is where all three are surfaced. Keeping the
// choice here means it is stated once, not in a converter the caller owns.
func stopDisplayName(s models.Stop) string {
	return s.NameFR
}

type stopIdx int32

const noStop stopIdx = -1

type tripMeta struct {
	shortName      string
	routeShortName string
	headsign       string
}

type connection struct {
	// instance is unique per (trip pattern, service date) — used for
	// tripEntered bookkeeping while scanning.
	instance   int32
	meta       int32 // index into Index.tripMetas
	depStop    stopIdx
	depTime    int64
	arrStop    stopIdx
	arrTime    int64
	boardable  bool
	alightable bool
}

type footpath struct {
	to      stopIdx
	seconds int64
}

// defaultMinTransferSeconds is used at a same-station change (shared
// parent_station) with no explicit transfers.txt entry — platforms are
// separate stops, so this must never be 0 (issue #1391).
const defaultMinTransferSeconds = 180

// DefaultMinTransferSeconds is defaultMinTransferSeconds exported for callers
// that need the same fallback before an Index exists (issue #1395).
const DefaultMinTransferSeconds = defaultMinTransferSeconds

// Index is the built, queryable in-memory router state for one rolling
// window of service days.
type Index struct {
	loc         *time.Location
	epoch       time.Time // local midnight of the window's first day
	stops       []models.Stop
	stopByID    map[string]stopIdx
	tripMetas   []tripMeta
	connections []connection // sorted by depTime ascending
	footpaths   map[stopIdx][]footpath
	// explicitPair records transfers.txt pairs (including "not possible")
	// so the default same-parent-station footpath never overrides them.
	explicitPair map[[2]stopIdx]bool
}

// Build assembles an Index from the caller's already-loaded rolling window,
// straight off the trains domain models — trains is this package's only
// caller, so there is no adapter layer in between. loc is the feed's local
// timezone (Europe/Brussels); windowStart anchors abs-second 0 to that
// date's local midnight. stopTimes may arrive in any order; Build groups
// them by trip_id and keeps each trip's rows in stop_sequence order.
func Build(
	loc *time.Location,
	windowStart time.Time,
	stops []models.Stop,
	transfers []models.Transfer,
	instances []models.ActiveTrip,
	stopTimes []models.StopTime,
) *Index {
	epoch := time.Date(
		windowStart.Year(), windowStart.Month(), windowStart.Day(),
		0, 0, 0, 0, loc,
	)
	idx := &Index{
		loc:          loc,
		epoch:        epoch,
		stops:        stops,
		stopByID:     make(map[string]stopIdx, len(stops)),
		tripMetas:    nil,
		connections:  nil,
		footpaths:    make(map[stopIdx][]footpath),
		explicitPair: make(map[[2]stopIdx]bool),
	}
	for i, s := range stops {
		idx.stopByID[s.StopID] = stopIdx(i)
	}

	idx.buildConnections(instances, groupStopTimesByTrip(stopTimes))
	idx.buildTransfers(transfers)
	idx.buildDefaultFootpaths()

	sort.Slice(idx.connections, func(i, j int) bool {
		return idx.connections[i].depTime < idx.connections[j].depTime
	})

	return idx
}

// groupStopTimesByTrip buckets stop_times by trip_id, sorting each bucket by
// stop_sequence so Build does not depend on the order the caller loaded them
// in.
func groupStopTimesByTrip(stopTimes []models.StopTime) map[string][]models.StopTime {
	out := make(map[string][]models.StopTime)
	for _, st := range stopTimes {
		out[st.TripID] = append(out[st.TripID], st)
	}
	for _, pattern := range out {
		sort.Slice(pattern, func(i, j int) bool {
			return pattern[i].StopSequence < pattern[j].StopSequence
		})
	}
	return out
}

func (idx *Index) buildConnections(
	instances []models.ActiveTrip,
	stopTimesByTripID map[string][]models.StopTime,
) {
	metaByKey := make(map[[3]string]int32)
	var nextInstance int32
	for _, inst := range instances {
		key := [3]string{inst.TripShortName, inst.RouteShortName, inst.TripHeadsign}
		metaIdx, ok := metaByKey[key]
		if !ok {
			//nolint:gosec //trip pattern count is small, never near int32 range
			metaIdx = int32(len(idx.tripMetas))
			idx.tripMetas = append(idx.tripMetas, tripMeta{
				shortName:      inst.TripShortName,
				routeShortName: inst.RouteShortName,
				headsign:       inst.TripHeadsign,
			})
			metaByKey[key] = metaIdx
		}

		pattern := stopTimesByTripID[inst.TripID]
		const minStopsForAConnection = 2
		if len(pattern) < minStopsForAConnection {
			continue
		}
		const hoursPerDay, secondsPerDay = 24, 86400
		dayAbs := int64(inst.Date.Sub(idx.epoch).Hours() / hoursPerDay * secondsPerDay)
		instance := nextInstance
		nextInstance++

		for i := 0; i+1 < len(pattern); i++ {
			from := pattern[i]
			to := pattern[i+1]
			fromIdx, ok1 := idx.stopByID[from.StopID]
			toIdx, ok2 := idx.stopByID[to.StopID]
			if !ok1 || !ok2 {
				continue
			}
			idx.connections = append(idx.connections, connection{
				instance:   instance,
				meta:       metaIdx,
				depStop:    fromIdx,
				depTime:    dayAbs + int64(from.DepartureSeconds),
				arrStop:    toIdx,
				arrTime:    dayAbs + int64(to.ArrivalSeconds),
				boardable:  from.PickupType != 1,
				alightable: to.DropOffType != 1,
			})
		}
	}
}

func (idx *Index) buildTransfers(transfers []models.Transfer) {
	const notPossible = 3
	const timed = 1
	for _, t := range transfers {
		from, ok1 := idx.stopByID[t.FromStopID]
		to, ok2 := idx.stopByID[t.ToStopID]
		if !ok1 || !ok2 || from == to {
			continue
		}
		idx.explicitPair[[2]stopIdx{from, to}] = true
		if t.TransferType == notPossible {
			continue
		}
		seconds := int64(defaultMinTransferSeconds)
		switch {
		case t.TransferType == timed:
			seconds = 0
		case t.MinTransferTime != nil:
			seconds = int64(*t.MinTransferTime)
		}
		idx.footpaths[from] = append(
			idx.footpaths[from],
			footpath{to: to, seconds: seconds},
		)
	}
}

// buildDefaultFootpaths adds a minimum-transfer-time edge between every
// pair of distinct stops sharing a parent_station that transfers.txt
// didn't already cover (explicitly, in either direction) — the same
// station never grants a free 0-minute change between platforms
// (issue #1391).
func (idx *Index) buildDefaultFootpaths() {
	byParent := make(map[string][]stopIdx)
	for i, s := range idx.stops {
		if s.ParentStation == "" {
			continue
		}
		byParent[s.ParentStation] = append(byParent[s.ParentStation], stopIdx(i))
	}
	for _, group := range byParent {
		for _, a := range group {
			for _, b := range group {
				if a == b {
					continue
				}
				if idx.explicitPair[[2]stopIdx{a, b}] {
					continue
				}
				idx.footpaths[a] = append(
					idx.footpaths[a],
					footpath{to: b, seconds: defaultMinTransferSeconds},
				)
			}
		}
	}
}

// MinTransferSeconds reports the minimum time a passenger needs to change
// from fromStopID to toStopID: the explicit transfers.txt footpath if one
// exists, otherwise defaultMinTransferSeconds. It never returns 0 for two
// distinct stops — the same station never grants a free platform change
// (issue #1391) — and is used by the live journey page to tell whether a
// delayed arrival still leaves the connection makeable (issue #1395).
func (idx *Index) MinTransferSeconds(fromStopID, toStopID string) int {
	if fromStopID == toStopID {
		return 0
	}
	from, ok1 := idx.stopByID[fromStopID]
	to, ok2 := idx.stopByID[toStopID]
	if ok1 && ok2 {
		for _, fp := range idx.footpaths[from] {
			if fp.to == to {
				return int(fp.seconds)
			}
		}
	}
	return defaultMinTransferSeconds
}

// toAbs converts a wall-clock time to the index's abs-second scale.
func (idx *Index) toAbs(t time.Time) int64 {
	t = t.In(idx.loc)
	return int64(t.Sub(idx.epoch).Seconds())
}

// fromAbs converts an abs-second value back to a wall-clock time.
func (idx *Index) fromAbs(sec int64) time.Time {
	return idx.epoch.Add(time.Duration(sec) * time.Second)
}

// resolveStops expands a station or platform id: a station (any stop that
// other stops declare as their parent_station) resolves to all of its
// platform children; a platform id resolves to itself; an unknown id
// resolves to nothing.
func (idx *Index) resolveStops(id string) []stopIdx {
	var out []stopIdx
	if self, ok := idx.stopByID[id]; ok {
		out = append(out, self)
	}
	for i, s := range idx.stops {
		if s.ParentStation == id {
			out = append(out, stopIdx(i))
		}
	}
	return out
}

func (idx *Index) stopByIdx(i stopIdx) models.Stop {
	return idx.stops[i]
}
