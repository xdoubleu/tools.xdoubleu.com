package csa

import (
	"errors"
	"math"
	"time"
)

// maxTransfersSearched bounds how many extra trips a search will try
// before giving up on finding a strictly-better (fewer transfers) option —
// 652 stops does not need more, and the page this feeds only needs a
// handful of alternatives (issue #1391).
const maxTransfersSearched = 3

// maxJourneyResults caps the Pareto set returned to callers.
const maxJourneyResults = 5

// arriveByLookback bounds how far before an "arrive by" request the search
// starts scanning connections from.
const arriveByLookback = 3 * time.Hour

// searchHorizon bounds how far past the requested time the search looks
// for a journey — this is "a window around the requested time" (issue
// #1391), not an open-ended search of the whole rolling window: a station
// pair with no service today should come back empty rather than surface an
// option days later.
const searchHorizon = 20 * time.Hour

// ErrUnknownStop is returned when the origin or destination stop id isn't
// in the index.
var ErrUnknownStop = errors.New("trains: unknown stop id")

// Leg is one boarded train, from where the passenger gets on to where they
// get off. TripShortName/RouteShortName identify the train to a user —
// nothing here is a trip_id.
type Leg struct {
	TripShortName  string
	RouteShortName string
	Headsign       string
	BoardStopID    string
	BoardStopName  string
	BoardPlatform  string
	BoardTime      time.Time
	AlightStopID   string
	AlightStopName string
	AlightPlatform string
	AlightTime     time.Time
}

// Journey is one ordered itinerary from origin to destination.
type Journey struct {
	Legs          []Leg
	DepartureTime time.Time
	ArrivalTime   time.Time
	Transfers     int
}

const noInt64 = int64(math.MaxInt64)

type hop struct {
	valid      bool
	isOrigin   bool
	isFootpath bool
	conn       connection
	from       stopIdx
}

// SearchJourneys returns a Pareto set over (arrival time, transfer count)
// for journeys from originID to destID: the earliest arrival, plus slower
// options with fewer changes, capped at maxJourneyResults. when is either
// a departure time (arriveBy=false) or a desired arrival time
// (arriveBy=true, approximated by scanning a lookback window ending at
// when and keeping only results that land on or before it).
func (idx *Index) SearchJourneys(
	originID, destID string, when time.Time, arriveBy bool,
) ([]Journey, error) {
	origins := idx.resolveStops(originID)
	dests := idx.resolveStops(destID)
	if len(origins) == 0 {
		return nil, ErrUnknownStop
	}
	if len(dests) == 0 {
		return nil, ErrUnknownStop
	}

	searchFrom := when
	if arriveBy {
		searchFrom = when.Add(-arriveByLookback)
	}
	startAbs := idx.toAbs(searchFrom)
	deadlineAbs := idx.toAbs(searchFrom) + int64(searchHorizon.Seconds())
	if arriveBy {
		deadlineAbs = idx.toAbs(when)
	}

	numStops := len(idx.stops)
	arrival := make([]int64, numStops)
	parent := make([]hop, numStops)
	for i := range arrival {
		arrival[i] = noInt64
	}
	for _, o := range origins {
		arrival[o] = startAbs
		//nolint:exhaustruct //conn/from are meaningless for an origin hop
		parent[o] = hop{valid: true, isOrigin: true}
	}
	idx.relaxFootpaths(arrival, parent, origins)

	var journeys []Journey
	bestArrival := noInt64

	for p := 0; p <= maxTransfersSearched; p++ {
		idx.scanPass(arrival, parent, startAbs, deadlineAbs)

		best, bestStop := bestAmong(arrival, dests)
		if bestStop == noStop || best >= bestArrival {
			continue
		}
		bestArrival = best
		j := idx.reconstruct(parent, bestStop)
		if len(j.Legs) == 0 {
			continue
		}
		journeys = append(journeys, j)
	}

	journeys = paretoFilter(journeys)
	if len(journeys) > maxJourneyResults {
		journeys = journeys[:maxJourneyResults]
	}
	return journeys, nil
}

// scanPass runs one Connection Scan sweep in place over arrival/parent,
// allowing at most one additional trip beyond whatever arrival/parent
// already reflect from the previous pass — see SearchJourneys' doc comment
// for why repeating this bounded sweep yields a Pareto frontier over
// (arrival time, transfer count).
func (idx *Index) scanPass(
	arrival []int64, parent []hop, startAbs, deadlineAbs int64,
) {
	entered := make(map[int32]bool)
	for _, c := range idx.connections {
		if c.depTime < startAbs {
			continue
		}
		if c.depTime > deadlineAbs {
			break
		}
		if !entered[c.instance] {
			if !c.boardable || arrival[c.depStop] > c.depTime {
				continue
			}
			entered[c.instance] = true
		}
		if !c.alightable || c.arrTime >= arrival[c.arrStop] {
			continue
		}
		arrival[c.arrStop] = c.arrTime
		//nolint:exhaustruct //isFootpath/isOrigin are false for a connection hop
		parent[c.arrStop] = hop{valid: true, conn: c, from: c.depStop}
		idx.relaxFootpaths(arrival, parent, []stopIdx{c.arrStop})
	}
}

func bestAmong(arrival []int64, dests []stopIdx) (int64, stopIdx) {
	best := noInt64
	bestStop := noStop
	for _, d := range dests {
		if arrival[d] < best {
			best = arrival[d]
			bestStop = d
		}
	}
	return best, bestStop
}

// relaxFootpaths propagates arrival[from]+walkTime to footpath-adjacent
// stops, for every from in seeds — called both to seed origin platforms
// and, inline during the connection scan, whenever a stop's arrival
// improves, so a same-station change is available to later connections in
// the same pass.
func (idx *Index) relaxFootpaths(arrival []int64, parent []hop, seeds []stopIdx) {
	for _, from := range seeds {
		base := arrival[from]
		if base == noInt64 {
			continue
		}
		for _, fp := range idx.footpaths[from] {
			cand := base + fp.seconds
			if cand < arrival[fp.to] {
				arrival[fp.to] = cand
				//nolint:exhaustruct //conn is meaningless for a footpath hop
				parent[fp.to] = hop{valid: true, isFootpath: true, from: from}
			}
		}
	}
}

// reconstruct walks the parent chain from dest back to an origin hop,
// merging consecutive connections that share an instance into one Leg.
func (idx *Index) reconstruct(parent []hop, dest stopIdx) Journey {
	type step struct {
		conn       connection
		isFootpath bool
	}
	var steps []step
	cur := dest
	for {
		h := parent[cur]
		if !h.valid || h.isOrigin {
			break
		}
		if h.isFootpath {
			//nolint:exhaustruct //conn is meaningless for a footpath step
			steps = append(steps, step{isFootpath: true})
			cur = h.from
			continue
		}
		//nolint:exhaustruct //isFootpath defaults false for a connection step
		steps = append(steps, step{conn: h.conn})
		cur = h.from
	}
	// reverse
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}

	var legs []Leg
	for _, st := range steps {
		if st.isFootpath {
			continue
		}
		if n := len(legs); n > 0 && legs[n-1].sameInstance(st.conn, idx) {
			legs[n-1].AlightStopID = idx.stopByIdx(st.conn.arrStop).ID
			legs[n-1].AlightStopName = idx.stopByIdx(st.conn.arrStop).Name
			legs[n-1].AlightPlatform = idx.stopByIdx(st.conn.arrStop).PlatformCode
			legs[n-1].AlightTime = idx.fromAbs(st.conn.arrTime)
			continue
		}
		meta := idx.tripMetas[st.conn.meta]
		board := idx.stopByIdx(st.conn.depStop)
		alight := idx.stopByIdx(st.conn.arrStop)
		legs = append(legs, Leg{
			TripShortName:  meta.shortName,
			RouteShortName: meta.routeShortName,
			Headsign:       meta.headsign,
			BoardStopID:    board.ID,
			BoardStopName:  board.Name,
			BoardPlatform:  board.PlatformCode,
			BoardTime:      idx.fromAbs(st.conn.depTime),
			AlightStopID:   alight.ID,
			AlightStopName: alight.Name,
			AlightPlatform: alight.PlatformCode,
			AlightTime:     idx.fromAbs(st.conn.arrTime),
		})
	}
	if len(legs) == 0 {
		//nolint:exhaustruct //no journey found; every field is its zero value
		return Journey{}
	}
	return Journey{
		Legs:          legs,
		DepartureTime: legs[0].BoardTime,
		ArrivalTime:   legs[len(legs)-1].AlightTime,
		Transfers:     len(legs) - 1,
	}
}

// sameInstance reports whether appending conn to this leg would just be
// riding the same train onward — matched on trip identity, held only
// inside the search, never persisted (issue #1391 / #1388's trip_id rule).
// The metadata-index match used here is what the caller sees; the actual
// per-day instance identity is checked by the caller not re-entering
// footpaths between elementary connections of the same trip.
func (l Leg) sameInstance(c connection, idx *Index) bool {
	return l.AlightStopID == idx.stopByIdx(c.depStop).ID &&
		l.TripShortName == idx.tripMetas[c.meta].shortName &&
		l.RouteShortName == idx.tripMetas[c.meta].routeShortName
}

// paretoFilter keeps only journeys where no other returned journey both
// arrives no later and uses no more transfers, sorted by arrival time.
func paretoFilter(journeys []Journey) []Journey {
	var out []Journey
	for _, j := range journeys {
		dominated := false
		for _, k := range journeys {
			if k.ArrivalTime.Before(j.ArrivalTime) && k.Transfers <= j.Transfers {
				dominated = true
				break
			}
			if k.ArrivalTime.Equal(j.ArrivalTime) && k.Transfers < j.Transfers {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, j)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ArrivalTime.Before(out[j-1].ArrivalTime); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
