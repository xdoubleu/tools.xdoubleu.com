package services

import (
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// overlayCalls returns one effective call per planned stop (zero = no live
// data). Calls are placed by station, using stop_sequence only when it agrees:
// it is optional in GTFS-RT and needn't match the static trip's numbering.
// Stops without a call inherit the last published delay, as GTFS-RT
// specifies; NO_DATA ends that propagation and SKIPPED is passed over.
func overlayCalls(
	pattern []models.StopTime, calls []models.StopCall, date time.Time,
) []models.StopCall {
	out := make([]models.StopCall, len(pattern))
	if len(calls) == 0 {
		return out
	}

	placed := placeCalls(pattern, calls)
	var carry *int
	for i, st := range pattern {
		call, ok := placed[i]
		if !ok {
			if carry != nil {
				out[i] = propagatedCall(st.StopID, *carry)
			}
			continue
		}
		call = withTimeDelays(call, st, date)
		out[i] = call
		switch call.State {
		case models.DelaySkipped:
			// The train doesn't call here; the delay carries past it.
		case models.DelayOnTime, models.DelayDelayed:
			carry = lastDelay(call)
		case models.DelayUnknown, models.DelayCancelled:
			carry = nil
		}
	}
	return out
}

// placeCalls maps pattern index to the call placed there; calls that match
// no remaining planned stop are dropped.
func placeCalls(
	pattern []models.StopTime, calls []models.StopCall,
) map[int]models.StopCall {
	bySeq := make(map[int]int, len(pattern))
	for i, st := range pattern {
		bySeq[st.StopSequence] = i
	}

	placed := make(map[int]models.StopCall, len(calls))
	next := 0
	for _, c := range calls {
		idx := placeCall(pattern, bySeq, c, next)
		if idx < 0 {
			continue
		}
		placed[idx] = c
		next = idx + 1
	}
	return placed
}

// placeCall returns c's index in pattern at or after next, or -1. A call
// without a stop_id can only be placed by its stop_sequence.
func placeCall(
	pattern []models.StopTime, bySeq map[int]int, c models.StopCall, next int,
) int {
	station := stationKey(c.StopID)
	if i, ok := bySeq[c.StopSequence]; ok && c.StopSequence > 0 && i >= next &&
		(c.StopID == "" || stationKey(pattern[i].StopID) == station) {
		return i
	}
	if c.StopID == "" {
		return -1
	}
	for i := next; i < len(pattern); i++ {
		if stationKey(pattern[i].StopID) == station {
			return i
		}
	}
	return -1
}

// stationKey identifies a stop's station across its platform ids.
func stationKey(stopID string) string {
	if uic := uicFromStopID(stopID); uic != "" {
		return uic
	}
	return stopID
}

// withTimeDelays derives delays a SCHEDULED call gave only as absolute times.
func withTimeDelays(
	call models.StopCall, st models.StopTime, date time.Time,
) models.StopCall {
	if call.State != models.DelayUnknown {
		return call
	}
	if call.ArrivalDelay == nil && call.ArrivalTime != nil {
		d := delayAgainst(*call.ArrivalTime, date, st.ArrivalSeconds)
		call.ArrivalDelay = &d
	}
	if call.DepartureDelay == nil && call.DepartureTime != nil {
		d := delayAgainst(*call.DepartureTime, date, st.DepartureSeconds)
		call.DepartureDelay = &d
	}
	call.State = stateForDelays(call.ArrivalDelay, call.DepartureDelay)
	return call
}

func delayAgainst(actual, date time.Time, scheduledSeconds int) int {
	scheduled := date.Add(time.Duration(scheduledSeconds) * time.Second)
	return int(actual.Sub(scheduled) / time.Second)
}

// stateForDelays: no delay published is unknown, any nonzero one is delayed.
func stateForDelays(arrival, departure *int) models.DelayState {
	switch {
	case arrival == nil && departure == nil:
		return models.DelayUnknown
	case (arrival != nil && *arrival != 0) || (departure != nil && *departure != 0):
		return models.DelayDelayed
	default:
		return models.DelayOnTime
	}
}

// lastDelay is the delay the train leaves a stop with.
func lastDelay(call models.StopCall) *int {
	if call.DepartureDelay != nil {
		return call.DepartureDelay
	}
	return call.ArrivalDelay
}

func propagatedCall(stopID string, delay int) models.StopCall {
	arr, dep := delay, delay
	return models.StopCall{
		StopID:         stopID,
		StopSequence:   0,
		State:          stateForDelays(&arr, &dep),
		ArrivalDelay:   &arr,
		DepartureDelay: &dep,
		ArrivalTime:    nil,
		DepartureTime:  nil,
	}
}
