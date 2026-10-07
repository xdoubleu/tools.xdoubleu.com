package services

import (
	"context"
	"sort"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// RealtimeTripView is what the realtime snapshot holds for one train today,
// next to the static trip journey detail overlays it on.
type RealtimeTripView struct {
	Snapshot models.Snapshot
	// StopCallStates counts every stop call in the snapshot by state.
	StopCallStates map[string]int
	// SnapshotShortNames are today's train numbers in the snapshot, sorted.
	SnapshotShortNames []string
	Found              bool
	Trip               models.TripUpdate
	// PlannedTripID is empty when no static trip runs the train today.
	PlannedTripID string
	PlannedStops  []models.StopTime
}

// RealtimeTrip reports the snapshot and, when shortName is set, its raw
// realtime trip for now's Brussels service date.
func (s *JourneyDetailService) RealtimeTrip(
	ctx context.Context, shortName string, now time.Time,
) (RealtimeTripView, error) {
	snapshot := s.realtime.Snapshot()
	states := make(map[string]int)
	for _, tu := range snapshot.Trips {
		for _, c := range tu.StopCalls {
			states[c.State.String()]++
		}
	}
	//nolint:exhaustruct //trip fields are filled below when shortName is set
	view := RealtimeTripView{Snapshot: snapshot, StopCallStates: states}
	day := serviceDateOf(now).Format("20060102")
	for key := range snapshot.Trips {
		if key.Date == day {
			view.SnapshotShortNames = append(view.SnapshotShortNames, key.ShortName)
		}
	}
	sort.Strings(view.SnapshotShortNames)
	if shortName == "" {
		return view, nil
	}

	date := serviceDateOf(now)
	view.Trip, view.Found = snapshot.CallFor(shortName, date)

	active, err := s.repos.Feed.TripByShortNameOnDate(ctx, shortName, date)
	if err != nil || active == nil {
		return view, err
	}
	view.PlannedTripID = active.TripID
	view.PlannedStops, err = s.repos.Feed.StopTimesForTrips(
		ctx, []string{active.TripID},
	)
	return view, err
}
