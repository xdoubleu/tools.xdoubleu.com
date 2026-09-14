package csa

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

// maxTransfersSearched bounds how many extra trips a search will try
// before giving up on finding a strictly-better (fewer transfers) option —
// 652 stops does not need more, and the page this feeds only needs a
// handful of alternatives (issue #1391).
const maxTransfersSearched = 3

// maxJourneyResults caps the total window returned to callers — enough for
// windowBefore + windowAfter (issue #1643) without ever trimming the window
// itself before the caller sees it.
const maxJourneyResults = 8

// windowBefore/windowAfter are how many distinct journeys SearchJourneys
// tries to surface strictly before/after the requested time, in addition to
// whatever searchOnce already finds at the requested time itself — issue
// #1643's "show a few options around the filled out time", since a single
// Pareto-optimal result is not what a real journey-search UI shows.
const windowBefore = 3
const windowAfter = 3

// beforeAnchorOffsets are how far back searchOnce is re-run, in order, to
// find windowBefore distinct earlier departures/arrivals. CSA is a
// forward-only algorithm, so there is no backward search — probing from
// progressively earlier anchor times and keeping what lands before the
// requested time is the practical alternative.
var beforeAnchorOffsets = []time.Duration{ //nolint:gochecknoglobals //readonly config
	1 * time.Hour, 2 * time.Hour, 4 * time.Hour,
}

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

// SearchJourneys returns a window of journeys from originID to destID
// around when: whatever searchOnce finds at when itself, plus up to
// windowBefore earlier and windowAfter later distinct journeys (issue
// #1643), merged, deduplicated and sorted by departure time (arrival time
// for arriveBy). For arriveBy=true only the "before" side is widened — a
// journey arriving after the requested deadline contradicts the search
// intent, so no "after" probing happens in that mode.
func (idx *Index) SearchJourneys(
	originID, destID string, when time.Time, arriveBy bool,
) ([]Journey, error) {
	primary, err := idx.searchOnce(originID, destID, when, arriveBy)
	if err != nil {
		return nil, err
	}

	w := newJourneyWindow(len(primary))
	w.add(primary)
	idx.widenBefore(w, originID, destID, when, arriveBy)
	if !arriveBy {
		idx.widenAfter(w, originID, destID, when)
	}

	sort.Slice(w.all, func(i, j int) bool {
		if arriveBy {
			return w.all[i].ArrivalTime.Before(w.all[j].ArrivalTime)
		}
		return w.all[i].DepartureTime.Before(w.all[j].DepartureTime)
	})
	if len(w.all) > maxJourneyResults {
		w.all = w.all[:maxJourneyResults]
	}
	return w.all, nil
}

// widenBefore probes progressively earlier anchor times (beforeAnchorOffsets)
// until w holds windowBefore distinct journeys strictly before when, or the
// anchors run out.
func (idx *Index) widenBefore(
	w *journeyWindow, originID, destID string, when time.Time, arriveBy bool,
) {
	before := func(j Journey) bool {
		if arriveBy {
			return j.ArrivalTime.Before(when)
		}
		return j.DepartureTime.Before(when)
	}
	for _, offset := range beforeAnchorOffsets {
		if w.count(before) >= windowBefore {
			return
		}
		probe, err := idx.searchOnce(originID, destID, when.Add(-offset), arriveBy)
		if err != nil {
			continue
		}
		w.add(filterJourneys(probe, before))
	}
}

// widenAfter probes progressively later anchor times (beforeAnchorOffsets,
// reused as a forward offset here) until w holds windowAfter distinct
// journeys departing at/after when (on top of whatever searchOnce already
// found there), or the anchors run out. Only called for arriveBy=false — an
// "after" journey has no meaning relative to an arrival deadline.
func (idx *Index) widenAfter(
	w *journeyWindow,
	originID, destID string,
	when time.Time,
) {
	atOrAfter := func(j Journey) bool { return !j.DepartureTime.Before(when) }
	for _, offset := range beforeAnchorOffsets {
		if w.count(atOrAfter) >= windowAfter+1 {
			return
		}
		probe, err := idx.searchOnce(originID, destID, when.Add(offset), false)
		if err != nil {
			continue
		}
		w.add(probe)
	}
}

func filterJourneys(js []Journey, keep func(Journey) bool) []Journey {
	var out []Journey
	for _, j := range js {
		if keep(j) {
			out = append(out, j)
		}
	}
	return out
}

// journeyWindow accumulates journeys found from more than one search
// anchor, deduplicating by journeyKey.
type journeyWindow struct {
	all  []Journey
	seen map[string]bool
}

func newJourneyWindow(capHint int) *journeyWindow {
	return &journeyWindow{
		all:  make([]Journey, 0, capHint),
		seen: make(map[string]bool, capHint),
	}
}

func (w *journeyWindow) add(js []Journey) {
	for _, j := range js {
		k := journeyKey(j)
		if w.seen[k] {
			continue
		}
		w.seen[k] = true
		w.all = append(w.all, j)
	}
}

func (w *journeyWindow) count(match func(Journey) bool) int {
	n := 0
	for _, j := range w.all {
		if match(j) {
			n++
		}
	}
	return n
}

// journeyKey is a stable dedup key for a Journey found from more than one
// search anchor — mirrors what services.EncodeJourneyID keys on (the leg
// sequence), built here from Journey's own fields since pkg/csa cannot
// import the services package.
func journeyKey(j Journey) string {
	var b strings.Builder
	for _, l := range j.Legs {
		b.WriteString(l.TripShortName)
		b.WriteByte('|')
		b.WriteString(l.BoardStopID)
		b.WriteByte('|')
		b.WriteString(l.BoardTime.Format(time.RFC3339))
		b.WriteByte('|')
		b.WriteString(l.AlightStopID)
		b.WriteByte('|')
		b.WriteString(l.AlightTime.Format(time.RFC3339))
		b.WriteByte(';')
	}
	return b.String()
}

// searchOnce returns a Pareto set over (arrival time, transfer count) for
// journeys from originID to destID: the earliest arrival, plus slower
// options with fewer changes. when is either a departure time
// (arriveBy=false) or a desired arrival time (arriveBy=true, approximated
// by scanning a lookback window ending at when and keeping only results
// that land on or before it). SearchJourneys calls this once at when and
// again at a handful of earlier/later anchor times to build a window.
func (idx *Index) searchOnce(
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
			arr := idx.stopByIdx(st.conn.arrStop)
			legs[n-1].AlightStopID = arr.StopID
			legs[n-1].AlightStopName = stopDisplayName(arr)
			legs[n-1].AlightPlatform = arr.PlatformCode
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
			BoardStopID:    board.StopID,
			BoardStopName:  stopDisplayName(board),
			BoardPlatform:  board.PlatformCode,
			BoardTime:      idx.fromAbs(st.conn.depTime),
			AlightStopID:   alight.StopID,
			AlightStopName: stopDisplayName(alight),
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
	return l.AlightStopID == idx.stopByIdx(c.depStop).StopID &&
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
