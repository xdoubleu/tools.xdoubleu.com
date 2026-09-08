package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// onTimeThresholdSeconds is the delay magnitude below which a call renders
// as "on time" rather than "delayed by N" — small schedule-adherence noise
// isn't worth alarming a passenger over (issue #1394).
const onTimeThresholdSeconds = 60

// JourneyDetailService rebuilds the full live state of a previously-searched
// journey (issue #1394): every leg's complete stop pattern between board and
// alight, each call's scheduled and (if published) live time, and any
// alerts attached to the leg's trip/route/stops. Nothing here is persisted
// — a journey_id round-trips everything needed to rebuild it (see
// EncodeJourneyID/DecodeJourneyID).
type JourneyDetailService struct {
	repos    *repositories.Repositories
	realtime *RealtimeService
}

func NewJourneyDetailService(
	repos *repositories.Repositories, realtime *RealtimeService,
) *JourneyDetailService {
	return &JourneyDetailService{repos: repos, realtime: realtime}
}

// GetJourneyDetail decodes journeyID and rebuilds the current live state of
// each leg it names.
func (s *JourneyDetailService) GetJourneyDetail(
	ctx context.Context, journeyID string,
) (*models.JourneyDetail, error) {
	refs, err := DecodeJourneyID(journeyID)
	if err != nil {
		return nil, err
	}

	snapshot := s.realtime.Snapshot()
	legs := make([]models.LegDetail, len(refs))
	for i, ref := range refs {
		leg, buildErr := s.buildLeg(ctx, ref, snapshot)
		if buildErr != nil {
			return nil, buildErr
		}
		legs[i] = leg
	}

	return &models.JourneyDetail{
		Legs:          legs,
		DepartureTime: refs[0].BoardTime,
		ArrivalTime:   refs[len(refs)-1].AlightTime,
	}, nil
}

func (s *JourneyDetailService) buildLeg(
	ctx context.Context, ref LegRef, snapshot models.Snapshot,
) (models.LegDetail, error) {
	date := serviceDateOf(ref.BoardTime)
	active, err := s.repos.Feed.TripByShortNameOnDate(ctx, ref.TripShortName, date)
	if err != nil {
		return models.LegDetail{}, err
	}
	if active == nil {
		// The static timetable no longer carries this trip (outside the
		// router's rolling window, or the feed changed) — render it with no
		// stop detail rather than failing the whole journey.
		//nolint:exhaustruct //no stop detail is known for a trip outside the window
		return models.LegDetail{TripShortName: ref.TripShortName}, nil
	}

	stopTimes, err := s.repos.Feed.StopTimesForTrips(ctx, []string{active.TripID})
	if err != nil {
		return models.LegDetail{}, err
	}
	boardIdx, alightIdx := legRange(stopTimes, ref.BoardStopID, ref.AlightStopID)
	if boardIdx < 0 || alightIdx < 0 || alightIdx < boardIdx {
		//nolint:exhaustruct //board/alight stop not found in this trip's pattern
		return models.LegDetail{
			TripShortName:  ref.TripShortName,
			RouteShortName: active.RouteShortName,
			Headsign:       active.TripHeadsign,
		}, nil
	}
	pattern := stopTimes[boardIdx : alightIdx+1]

	stops, err := s.repos.Feed.StopsByIDs(ctx, stopIDsOf(pattern))
	if err != nil {
		return models.LegDetail{}, err
	}

	tripUpdate, hasTripUpdate := snapshot.CallFor(ref.TripShortName, date)
	cancelled := hasTripUpdate && tripUpdate.State == models.DelayCancelled

	return models.LegDetail{
		TripShortName:  ref.TripShortName,
		RouteShortName: active.RouteShortName,
		Headsign:       active.TripHeadsign,
		Cancelled:      cancelled,
		Stops: buildStopDetails(
			pattern, stopByID(stops), tripUpdate.StopCalls, cancelled, date, ref,
		),
		Alerts: alertsForLeg(snapshot.Alerts, active, stopIDsOf(pattern)),
	}, nil
}

func buildStopDetails(
	pattern []models.StopTime,
	stops map[string]models.Stop,
	calls []models.StopCall,
	cancelled bool,
	date time.Time,
	ref LegRef,
) []models.StopDetail {
	callBySeq := make(map[int]models.StopCall, len(calls))
	for _, c := range calls {
		callBySeq[c.StopSequence] = c
	}

	details := make([]models.StopDetail, len(pattern))
	for i, st := range pattern {
		info := stops[st.StopID]
		arr := date.Add(time.Duration(st.ArrivalSeconds) * time.Second)
		dep := date.Add(time.Duration(st.DepartureSeconds) * time.Second)
		//nolint:exhaustruct //ArrivalDelay/DepartureDelay set below by applyLiveState
		d := models.StopDetail{
			StopID:             st.StopID,
			StopName:           info.NameFR,
			Platform:           info.PlatformCode,
			ScheduledArrival:   &arr,
			ScheduledDeparture: &dep,
			State:              models.DelayUnknown,
			IsBoardStop:        st.StopID == ref.BoardStopID,
			IsAlightStop:       st.StopID == ref.AlightStopID,
		}
		applyLiveState(&d, callBySeq[st.StopSequence], cancelled)
		details[i] = d
	}
	return details
}

// applyLiveState overlays call onto d. When no call was decoded for this
// stop_sequence, call is the zero value, whose State is DelayUnknown — the
// same "no live data" state d already started at.
func applyLiveState(d *models.StopDetail, call models.StopCall, cancelled bool) {
	if cancelled {
		d.State = models.DelayCancelled
		return
	}

	d.State = call.State
	d.ArrivalDelay = call.ArrivalDelay
	d.DepartureDelay = call.DepartureDelay
	if d.State == models.DelayDelayed && !exceedsThreshold(call) {
		// jitter under the threshold renders as on-time, not "delayed by 0".
		d.State = models.DelayOnTime
	}
}

func exceedsThreshold(call models.StopCall) bool {
	if call.ArrivalDelay != nil &&
		absInt(*call.ArrivalDelay) >= onTimeThresholdSeconds {
		return true
	}
	if call.DepartureDelay != nil &&
		absInt(*call.DepartureDelay) >= onTimeThresholdSeconds {
		return true
	}
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// serviceDateOf returns the Brussels-local midnight of t's day — GTFS
// stop_times are wall-clock offsets from a service date's midnight.
func serviceDateOf(t time.Time) time.Time {
	local := t.In(brusselsLoc)
	return time.Date(
		local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, brusselsLoc,
	)
}

// legRange finds the index range [boardIdx, alightIdx] within pattern
// covering the passenger's boarded portion of the trip — a trip's
// stop_times commonly continue past where this leg's passenger alights.
func legRange(
	pattern []models.StopTime, boardStopID, alightStopID string,
) (int, int) {
	boardIdx, alightIdx := -1, -1
	for i, st := range pattern {
		if st.StopID == boardStopID && boardIdx == -1 {
			boardIdx = i
		}
		if st.StopID == alightStopID {
			alightIdx = i
		}
	}
	return boardIdx, alightIdx
}

func stopIDsOf(pattern []models.StopTime) []string {
	ids := make([]string, len(pattern))
	for i, st := range pattern {
		ids[i] = st.StopID
	}
	return ids
}

func stopByID(stops []models.Stop) map[string]models.Stop {
	out := make(map[string]models.Stop, len(stops))
	for _, s := range stops {
		out[s.StopID] = s
	}
	return out
}

func alertsForLeg(
	alerts []models.Alert, active *repositories.ActiveTrip, stopIDs []string,
) []models.Alert {
	stopSet := make(map[string]bool, len(stopIDs))
	for _, id := range stopIDs {
		stopSet[id] = true
	}
	var out []models.Alert
	for _, a := range alerts {
		if containsStr(a.InformedTripIDs, active.TripID) ||
			containsStr(a.InformedRouteIDs, active.RouteID) ||
			anyInSet(a.InformedStopIDs, stopSet) {
			out = append(out, a)
		}
	}
	return out
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func anyInSet(list []string, set map[string]bool) bool {
	for _, s := range list {
		if set[s] {
			return true
		}
	}
	return false
}
