package csa

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

// maxTransfersSearched bounds the extra trips tried when looking for
// fewer-transfer options.
const maxTransfersSearched = 3

// maxJourneyResults caps the returned window (windowBefore + windowAfter).
const maxJourneyResults = 8

// windowBefore/windowAfter: how many journeys to surface before/after the
// requested time, beyond what searchOnce finds at it.
const windowBefore = 3
const windowAfter = 3

// beforeAnchorOffsets are the earlier anchors searchOnce is re-run from.
// CSA only scans forward, so earlier options come from earlier anchors.
var beforeAnchorOffsets = []time.Duration{ //nolint:gochecknoglobals //readonly config
	1 * time.Hour, 2 * time.Hour, 4 * time.Hour,
}

const arriveByLookback = 3 * time.Hour

// searchHorizon bounds how far past the requested time to look, so a pair
// with no service today returns empty rather than days later.
const searchHorizon = 20 * time.Hour

// ErrUnknownStop is returned when the origin or destination stop id isn't
// in the index.
var ErrUnknownStop = errors.New("trains: unknown stop id")

// Leg is one boarded train. Identified by TripShortName/RouteShortName,
// never trip_id.
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

// SearchJourneys returns journeys around when: searchOnce's result plus up
// to windowBefore earlier and windowAfter later, deduped and sorted. arriveBy
// only widens before, since later arrivals miss the deadline.
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

// widenBefore probes earlier anchors until windowBefore journeys precede when.
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

// widenAfter probes later anchors (reusing beforeAnchorOffsets) until
// windowAfter journeys depart at/after when.
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

// journeyWindow accumulates journeys across anchors, deduped by journeyKey.
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

// journeyKey keys a Journey by its leg sequence, like EncodeJourneyID.
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

// searchOnce returns a Pareto set over (arrival, transfers). For arriveBy,
// it scans a lookback window ending at when and keeps results arriving by it.
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

// scanPass runs one Connection Scan sweep allowing one more trip than the
// previous pass; repeating it yields the Pareto frontier.
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

// relaxFootpaths propagates arrival[from]+walkTime to footpath neighbours of
// seeds; also called mid-scan so same-station changes are usable in the pass.
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

// reconstruct walks parents from dest, merging same-instance connections
// into one Leg.
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

// sameInstance reports whether conn continues this leg's train.
func (l Leg) sameInstance(c connection, idx *Index) bool {
	return l.AlightStopID == idx.stopByIdx(c.depStop).StopID &&
		l.TripShortName == idx.tripMetas[c.meta].shortName &&
		l.RouteShortName == idx.tripMetas[c.meta].routeShortName
}

// paretoFilter keeps journeys not dominated on (arrival, transfers), sorted
// by arrival.
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
