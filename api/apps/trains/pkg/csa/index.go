// Package csa is an in-memory Connection Scan Algorithm router over a rolling
// window of the GTFS timetable. trip_id is used only internally to group
// connections into legs; it's never returned.
package csa

import (
	"sort"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// stopDisplayName is the leg's station name: models.Stop.DisplayName, the
// same label the station search shows.
func stopDisplayName(s models.Stop) string {
	return s.DisplayName
}

type stopIdx int32

const noStop stopIdx = -1

type tripMeta struct {
	shortName      string
	routeShortName string
	headsign       string
}

type connection struct {
	// instance is unique per (trip pattern, service date).
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

// defaultMinTransferSeconds applies to same-station changes without a
// transfers.txt entry; platforms are separate stops, so never 0.
const defaultMinTransferSeconds = 180

// DefaultMinTransferSeconds exports the fallback for use before an Index
// exists.
const DefaultMinTransferSeconds = defaultMinTransferSeconds

// Index is the queryable router state for one rolling window.
type Index struct {
	loc         *time.Location
	epoch       time.Time // local midnight of the window's first day
	stops       []models.Stop
	stopByID    map[string]stopIdx
	tripMetas   []tripMeta
	connections []connection // sorted by depTime ascending
	footpaths   map[stopIdx][]footpath
	// explicitPair records transfers.txt pairs so default footpaths never
	// override them.
	explicitPair map[[2]stopIdx]bool
}

// Build assembles an Index from the loaded window. loc is the feed timezone;
// windowStart anchors abs-second 0 to that date's local midnight. stopTimes
// may be in any order.
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

// groupStopTimesByTrip buckets stop_times by trip_id, sorted by stop_sequence.
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
		// pgx scans DATE as UTC midnight; rebuild local midnight from Y/M/D, or every
		// connection would be off by the UTC offset.
		y, m, d := inst.Date.Date()
		tripLocalMidnight := time.Date(y, m, d, 0, 0, 0, 0, idx.loc)
		dayAbs := int64(tripLocalMidnight.Sub(idx.epoch).Seconds())
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

// buildDefaultFootpaths adds a min-transfer edge between stops sharing a
// parent_station not already covered by transfers.txt.
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

// MinTransferSeconds returns the transfers.txt time, else
// defaultMinTransferSeconds; never 0 for distinct stops.
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

func (idx *Index) toAbs(t time.Time) int64 {
	t = t.In(idx.loc)
	return int64(t.Sub(idx.epoch).Seconds())
}

func (idx *Index) fromAbs(sec int64) time.Time {
	return idx.epoch.Add(time.Duration(sec) * time.Second)
}

// resolveStops expands a station id to its platforms; a platform resolves to
// itself, an unknown id to nothing.
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
