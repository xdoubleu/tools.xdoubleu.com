package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

// finalArrivalSlipSeconds is how far the projected final arrival must slip
// past the planned one before that alone counts as the journey breaking. A
// smaller delay that still threatens no connection is shown on the page
// (issue #1394) but never escalated to a re-plan (issue #1395).
const finalArrivalSlipSeconds = 900

// replanner is the slice of JourneyService the alternative evaluator needs:
// re-query the timetable router from an arbitrary origin/time, and ask what a
// change between two stops costs.
type replanner interface {
	SearchJourneys(
		ctx context.Context,
		originStopID, destStopID string,
		when time.Time,
		arriveBy bool,
	) ([]csa.Journey, error)
	MinTransferSeconds(fromStopID, toStopID string) int
}

// journeyBreak is what detectJourneyBreak found: the passenger-facing reason,
// where and when a re-plan should start from, the original destination, and
// the trips already known to be dead so a timetable-only re-plan does not
// just hand them back.
type journeyBreak struct {
	reason       string
	fromStopID   string
	fromStopName string
	at           time.Time
	destStopID   string
	deadTrips    map[string]bool
}

// detectJourneyBreak walks the live legs in order and returns the first
// positive signal that the planned journey no longer works. Absence of live
// data (models.DelayUnknown) is never such a signal — liveArrival only
// yields a time for a DelayOnTime/DelayDelayed reading, so a NO_DATA leg
// contributes nothing here (issue #1395).
func detectJourneyBreak(
	refs []LegRef,
	legs []models.LegDetail,
	minTransfer func(fromStopID, toStopID string) int,
) (journeyBreak, bool) {
	if len(refs) == 0 || len(legs) != len(refs) {
		//nolint:exhaustruct //sentinel "no break" zero value
		return journeyBreak{}, false
	}
	destStopID := refs[len(refs)-1].AlightStopID

	for i := range legs {
		if brk, ok := brokenLeg(refs, legs, i, destStopID); ok {
			return brk, true
		}
		if i+1 < len(legs) {
			if brk, ok := missedConnection(
				refs, legs, i, destStopID, minTransfer,
			); ok {
				return brk, true
			}
		}
	}

	return finalArrivalBreak(refs, legs, destStopID)
}

// The planned board/alight times a break re-plans against come from the
// LegRef (what the router itself returned when the journey was first
// searched), never from the live overlay's scheduled times — the two are
// produced in different time frames and only the LegRef frame lines up with
// what a fresh SearchJourneys call expects. Realtime delays, being frame-
// independent second counts, are read off the overlay and added on.

// brokenLeg covers a leg that cannot be ridden as planned: the whole trip is
// cancelled, or the stop the passenger boards or alights at is skipped. The
// re-plan starts from where the passenger would have boarded that leg.
func brokenLeg(
	refs []LegRef, legs []models.LegDetail, i int, destStopID string,
) (journeyBreak, bool) {
	leg := legs[i]
	boardSkipped := stopStateAt(leg, refs[i].BoardStopID) == models.DelaySkipped
	alightSkipped := stopStateAt(leg, refs[i].AlightStopID) == models.DelaySkipped
	if !leg.Cancelled && !boardSkipped && !alightSkipped {
		//nolint:exhaustruct //sentinel "no break" zero value
		return journeyBreak{}, false
	}

	fromID, at, fromName := boardingPoint(refs, legs, i)
	return journeyBreak{
		reason:       brokenLegReason(leg, refs[i], boardSkipped, alightSkipped),
		fromStopID:   fromID,
		fromStopName: fromName,
		at:           at,
		destStopID:   destStopID,
		deadTrips:    map[string]bool{leg.TripShortName: true},
	}, true
}

// missedConnection fires when leg i's live arrival, plus the minimum change
// time, lands after leg i+1's live departure. Live arrival requires a
// positive reading on leg i's alight stop; a missing reading returns no
// break, which is what keeps NO_DATA from triggering one.
func missedConnection(
	refs []LegRef,
	legs []models.LegDetail,
	i int,
	destStopID string,
	minTransfer func(fromStopID, toStopID string) int,
) (journeyBreak, bool) {
	arrDelay, ok := arrivalDelayAt(legs[i], refs[i].AlightStopID)
	if !ok {
		//nolint:exhaustruct //sentinel "no break" zero value
		return journeyBreak{}, false
	}
	arr := refs[i].AlightTime.Add(time.Duration(arrDelay) * time.Second)
	depDelay := departureDelayAt(legs[i+1], refs[i+1].BoardStopID)
	dep := refs[i+1].BoardTime.Add(time.Duration(depDelay) * time.Second)

	gap := time.Duration(
		minTransfer(refs[i].AlightStopID, refs[i+1].BoardStopID),
	) * time.Second
	if !arr.Add(gap).After(dep) {
		//nolint:exhaustruct //sentinel "no break" zero value
		return journeyBreak{}, false
	}

	name := stopNameAt(legs[i], refs[i].AlightStopID)
	clock := departureClock(legs[i+1], refs[i+1].BoardStopID, depDelay, dep)
	return journeyBreak{
		reason:       missedConnectionReason(clock, name, arr.Add(gap).Sub(dep)),
		fromStopID:   refs[i].AlightStopID,
		fromStopName: name,
		at:           arr,
		destStopID:   destStopID,
		deadTrips:    map[string]bool{},
	}, true
}

// finalArrivalBreak fires when the last leg's live arrival at the
// destination has slipped at least finalArrivalSlipSeconds past the plan.
func finalArrivalBreak(
	refs []LegRef, legs []models.LegDetail, destStopID string,
) (journeyBreak, bool) {
	last := len(legs) - 1
	arrDelay, ok := arrivalDelayAt(legs[last], refs[last].AlightStopID)
	if !ok || arrDelay < finalArrivalSlipSeconds {
		//nolint:exhaustruct //sentinel "no break" zero value
		return journeyBreak{}, false
	}

	fromID, at, fromName := boardingPoint(refs, legs, last)
	return journeyBreak{
		reason: finalSlipReason(
			stopNameAt(legs[last], refs[last].AlightStopID),
			time.Duration(arrDelay)*time.Second,
		),
		fromStopID:   fromID,
		fromStopName: fromName,
		at:           at,
		destStopID:   destStopID,
		deadTrips:    map[string]bool{},
	}, true
}

// boardingPoint is the stop, time and station name a re-plan of leg i starts
// from: leg i's board stop, reached at the original departure time (a first
// leg) or at the previous leg's live arrival — its planned arrival shifted
// by whatever positive delay the overlay reports there.
func boardingPoint(
	refs []LegRef, legs []models.LegDetail, i int,
) (string, time.Time, string) {
	fromID := refs[i].BoardStopID
	fromName := stopNameAt(legs[i], fromID)
	if i == 0 {
		return fromID, refs[0].BoardTime, fromName
	}
	prevDelay, _ := arrivalDelayAt(legs[i-1], refs[i-1].AlightStopID)
	at := refs[i-1].AlightTime.Add(time.Duration(prevDelay) * time.Second)
	return fromID, at, fromName
}

// arrivalDelayAt returns the positive-signal arrival delay (seconds) for a
// stop: 0 for an on-time reading, the published delay for a delayed one, and
// (0, false) for NO_DATA / skipped / cancelled — the case that must never
// feed a re-plan.
func arrivalDelayAt(leg models.LegDetail, stopID string) (int, bool) {
	sd, ok := stopDetailAt(leg, stopID)
	if !ok {
		return 0, false
	}
	switch sd.State {
	case models.DelayOnTime:
		return 0, true
	case models.DelayDelayed:
		if sd.ArrivalDelay == nil {
			return 0, false
		}
		return *sd.ArrivalDelay, true
	case models.DelayUnknown, models.DelaySkipped, models.DelayCancelled:
		return 0, false
	default:
		return 0, false
	}
}

// departureDelayAt returns the published departure delay for a stop, or 0
// when none was published — the conservative assumption, since only a
// positive incoming delay drives a missed-connection verdict.
func departureDelayAt(leg models.LegDetail, stopID string) int {
	sd, ok := stopDetailAt(leg, stopID)
	if !ok || sd.State != models.DelayDelayed || sd.DepartureDelay == nil {
		return 0
	}
	return *sd.DepartureDelay
}

// departureClock is the wall-clock instant to show for a missed connection:
// the overlay's scheduled departure (which is in the passenger-facing time
// frame) plus the live delay, falling back to the LegRef-derived instant
// when the overlay carries no scheduled departure for that stop.
func departureClock(
	leg models.LegDetail, stopID string, depDelay int, fallback time.Time,
) time.Time {
	if sd, ok := stopDetailAt(leg, stopID); ok && sd.ScheduledDeparture != nil {
		return sd.ScheduledDeparture.Add(time.Duration(depDelay) * time.Second)
	}
	return fallback
}

func stopDetailAt(leg models.LegDetail, stopID string) (models.StopDetail, bool) {
	for _, sd := range leg.Stops {
		if sd.StopID == stopID {
			return sd, true
		}
	}
	//nolint:exhaustruct //sentinel "not found" zero value
	return models.StopDetail{}, false
}

func stopStateAt(leg models.LegDetail, stopID string) models.DelayState {
	if sd, ok := stopDetailAt(leg, stopID); ok {
		return sd.State
	}
	return models.DelayUnknown
}

func stopNameAt(leg models.LegDetail, stopID string) string {
	if sd, ok := stopDetailAt(leg, stopID); ok && sd.StopName != "" {
		return sd.StopName
	}
	return stopID
}

func brokenLegReason(
	leg models.LegDetail, ref LegRef, boardSkipped, alightSkipped bool,
) string {
	train := strings.TrimSpace(leg.RouteShortName + " " + leg.TripShortName)
	switch {
	case leg.Cancelled:
		return fmt.Sprintf("%s is cancelled", train)
	case boardSkipped:
		return fmt.Sprintf(
			"%s no longer stops at %s", train, stopNameAt(leg, ref.BoardStopID),
		)
	case alightSkipped:
		return fmt.Sprintf(
			"%s no longer stops at %s", train, stopNameAt(leg, ref.AlightStopID),
		)
	default:
		return fmt.Sprintf("%s is disrupted", train)
	}
}

func missedConnectionReason(dep time.Time, station string, by time.Duration) string {
	return fmt.Sprintf(
		"You'll miss the %s at %s by %s",
		dep.In(brusselsLoc).Format("15:04"), station, humanizeDuration(by),
	)
}

func finalSlipReason(destination string, slip time.Duration) string {
	return fmt.Sprintf(
		"Arrival at %s is running %s late", destination, humanizeDuration(slip),
	)
}

// humanizeDuration renders a positive duration as whole minutes, never "0
// min" — a sub-minute slip that reached here still rounds up to one.
func humanizeDuration(d time.Duration) string {
	mins := int((d + 30*time.Second) / time.Minute)
	if mins < 1 {
		mins = 1
	}
	return fmt.Sprintf("%d min", mins)
}

// evaluateAlternative runs the break detector and, when it fires, re-plans
// from the next reachable station. A re-plan that fails or finds nothing
// still returns the reason on its own — telling the passenger the journey is
// broken is useful even without a replacement (issue #1395).
func (s *JourneyDetailService) evaluateAlternative(
	ctx context.Context, refs []LegRef, legs []models.LegDetail,
) *models.JourneyAlternative {
	if s.replan == nil {
		return nil
	}
	brk, ok := detectJourneyBreak(refs, legs, s.replan.MinTransferSeconds)
	if !ok {
		return nil
	}

	alt := &models.JourneyAlternative{
		Reason:       brk.reason,
		FromStopName: brk.fromStopName,
		Journey:      nil,
	}

	journeys, err := s.replan.SearchJourneys(
		ctx, brk.fromStopID, brk.destStopID, brk.at, false,
	)
	if err != nil {
		return alt
	}
	for i := range journeys {
		if reusesDeadTrip(journeys[i], brk.deadTrips) {
			continue
		}
		alt.Journey = toJourneyOption(&journeys[i])
		break
	}
	return alt
}

func reusesDeadTrip(j csa.Journey, dead map[string]bool) bool {
	for _, l := range j.Legs {
		if dead[l.TripShortName] {
			return true
		}
	}
	return false
}

// toJourneyOption converts a router result into the model the journey-detail
// wire carries, stamping the same opaque journey_id SearchJourneys would —
// the id the page navigates to when the passenger adopts the alternative.
func toJourneyOption(j *csa.Journey) *models.JourneyOption {
	legs := make([]models.JourneyOptionLeg, len(j.Legs))
	refs := make([]LegRef, len(j.Legs))
	for i, l := range j.Legs {
		legs[i] = models.JourneyOptionLeg{
			TripShortName:  l.TripShortName,
			RouteShortName: l.RouteShortName,
			Headsign:       l.Headsign,
			BoardStopID:    l.BoardStopID,
			BoardStopName:  l.BoardStopName,
			BoardPlatform:  l.BoardPlatform,
			BoardTime:      l.BoardTime,
			AlightStopID:   l.AlightStopID,
			AlightStopName: l.AlightStopName,
			AlightPlatform: l.AlightPlatform,
			AlightTime:     l.AlightTime,
		}
		refs[i] = LegRef{
			TripShortName: l.TripShortName,
			BoardStopID:   l.BoardStopID,
			BoardTime:     l.BoardTime,
			AlightStopID:  l.AlightStopID,
			AlightTime:    l.AlightTime,
		}
	}
	return &models.JourneyOption{
		Legs:          legs,
		DepartureTime: j.DepartureTime,
		ArrivalTime:   j.ArrivalTime,
		Transfers:     j.Transfers,
		JourneyID:     EncodeJourneyID(refs),
	}
}
