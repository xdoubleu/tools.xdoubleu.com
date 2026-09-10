// Package csa tests. This file stays in-package (not csa_test) because it
// needs the unexported defaultMinTransferSeconds for a precise
// same-station-transfer boundary assertion.
//
//nolint:testpackage //see comment above
package csa

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

var loc = time.UTC //nolint:gochecknoglobals //test fixture timezone

// mkStop / mkTrip / mkST build the trains domain models csa.Build now takes
// directly, leaving the fields these tests don't exercise at their zero
// value.
//
//nolint:exhaustruct //test fixture: unset fields are deliberately zero
func mkStop(id, name, parent, platform string, locationType int) models.Stop {
	return models.Stop{
		StopID:        id,
		NameFR:        name,
		ParentStation: parent,
		PlatformCode:  platform,
		LocationType:  locationType,
	}
}

//nolint:exhaustruct //test fixture: unset fields are deliberately zero
func mkTrip(tripID, shortName, headsign string, date time.Time) models.ActiveTrip {
	return models.ActiveTrip{
		TripID:         tripID,
		TripShortName:  shortName,
		RouteShortName: "IC",
		TripHeadsign:   headsign,
		Date:           date,
	}
}

//nolint:exhaustruct //test fixture: TripID is filled in by flattenPatterns
func mkST(seq int, stopID string, arr, dep, pickup, dropOff int) models.StopTime {
	return models.StopTime{
		StopSequence:     seq,
		StopID:           stopID,
		ArrivalSeconds:   arr,
		DepartureSeconds: dep,
		PickupType:       pickup,
		DropOffType:      dropOff,
	}
}

// flattenPatterns turns a per-trip pattern map into the flat, TripID-tagged
// stop_times slice csa.Build consumes.
func flattenPatterns(patterns map[string][]models.StopTime) []models.StopTime {
	var out []models.StopTime
	for tripID, rows := range patterns {
		for _, r := range rows {
			r.TripID = tripID
			out = append(out, r)
		}
	}
	return out
}

// stations sharing a parent — mirrors the real feed's "S"+UIC station /
// child-platform shape (issue #1389/#1391).
func baseStops() []models.Stop {
	return []models.Stop{
		mkStop("SA", "Alpha", "", "", 1),
		mkStop("A1", "Alpha", "SA", "1", 0),
		mkStop("SB", "Bravo", "", "", 1),
		mkStop("B1", "Bravo", "SB", "1", 0),
		mkStop("B2", "Bravo", "SB", "2", 0),
		mkStop("SC", "Charlie", "", "", 1),
		mkStop("C1", "Charlie", "SC", "1", 0),
	}
}

func day(base time.Time, n int) time.Time { return base.AddDate(0, 0, n) }

func TestSearchJourneys_Direct(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	j := journeys[0]
	assert.Len(t, j.Legs, 1)
	assert.Equal(t, 0, j.Transfers)
	assert.Equal(t, "100", j.Legs[0].TripShortName)
	assert.Equal(t, "A1", j.Legs[0].BoardStopID)
	assert.Equal(t, "Alpha", j.Legs[0].BoardStopName)
	assert.Equal(t, "B1", j.Legs[0].AlightStopID)
	assert.Equal(t, day(window, 0).Add(8*time.Hour+20*time.Minute), j.ArrivalTime)
}

func TestSearchJourneys_RequiresTransfer(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []models.ActiveTrip{
		mkTrip("leg1", "100", "Bravo", day(window, 0)),
		mkTrip("leg2", "200", "Charlie", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"leg1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
		// same-station change B1 -> B2 must cost the default transfer time,
		// never a free 0-minute move (issue #1391's core trap): the gap
		// here is 300s, comfortably above defaultMinTransferSeconds.
		"leg2": {
			mkST(1, "B2", 8*3600+1500, 8*3600+1500, 0, 0),
			mkST(2, "C1", 8*3600+2700, 8*3600+2700, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	j := journeys[0]
	require.Len(t, j.Legs, 2)
	assert.Equal(t, 1, j.Transfers)
	assert.Equal(t, "B1", j.Legs[0].AlightStopID)
	assert.Equal(t, "B2", j.Legs[1].BoardStopID)

	// a same-station change with less than defaultMinTransferSeconds of
	// slack must NOT be offered at all.
	patterns["leg2"][0] = mkST(
		1, "B2",
		8*3600+1200+defaultMinTransferSeconds-1,
		8*3600+1200+defaultMinTransferSeconds-1,
		0, 0,
	)
	idx2 := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))
	journeys2, err := idx2.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	assert.Empty(t, journeys2, "should not offer a sub-minimum same-station transfer")
}

func TestSearchJourneys_AfterMidnightCrossesServiceDay(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []models.ActiveTrip{
		mkTrip("night", "900", "Bravo", day(window, 0)),
	}
	// departs 23:50 day 0, arrives 25:10 (day 0's GTFS clock) == 01:10 day 1.
	patterns := map[string][]models.StopTime{
		"night": {
			mkST(1, "A1", 23*3600+3000, 23*3600+3000, 0, 0),
			mkST(2, "B1", 25*3600+600, 25*3600+600, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 23, 0, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	arr := journeys[0].ArrivalTime
	assert.Equal(t, 2, arr.Day())
	assert.Equal(t, 1, arr.Hour())
	assert.Equal(t, 10, arr.Minute())
}

func TestSearchJourneys_NoServiceOnRequestedDate(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	// the only trip runs on day 5, but the search asks for day 0.
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 5)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 7, 0, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	assert.Empty(t, journeys)
}

func TestSearchJourneys_NonBoardingCallNotOfferedAsBoardingPoint(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := append(baseStops(), mkStop("M1", "Midway", "", "", 0))
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			// technical pass-through: neither boardable nor alightable.
			mkST(2, "M1", 8*3600+600, 8*3600+600, 1, 1),
			mkST(3, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 8, 5, 0, 0, loc)
	journeys, err := idx.SearchJourneys("M1", "SB", when, false)
	require.NoError(t, err)
	assert.Empty(t, journeys, "must not be able to board at a non-boarding call")

	// but riding through M1 on the original trip still works.
	when0 := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys0, err := idx.SearchJourneys("SA", "SB", when0, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys0)
	assert.Len(t, journeys0[0].Legs, 1)
}

func TestSearchJourneys_UnknownStop(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	idx := Build(loc, window, baseStops(), nil, nil, nil)
	_, err := idx.SearchJourneys("nope", "SB", window, false)
	require.ErrorIs(t, err, ErrUnknownStop)
	_, err = idx.SearchJourneys("SA", "nope", window, false)
	require.ErrorIs(t, err, ErrUnknownStop)
}

func TestSearchJourneys_ArriveBy(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	idx := Build(loc, window, stops, nil, instances, flattenPatterns(patterns))

	arriveBy := time.Date(2026, 10, 1, 9, 0, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", arriveBy, true)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	assert.True(
		t,
		journeys[0].ArrivalTime.Before(arriveBy) ||
			journeys[0].ArrivalTime.Equal(arriveBy),
	)
}

func TestSearchJourneys_ExplicitTransferNotPossible(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []models.ActiveTrip{
		mkTrip("leg1", "100", "Bravo", day(window, 0)),
		mkTrip("leg2", "200", "Charlie", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"leg1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
		"leg2": {
			mkST(1, "B2", 20*3600, 20*3600, 0, 0),
			mkST(2, "C1", 20*3600+1200, 20*3600+1200, 0, 0),
		},
	}
	const notPossible = 3
	transfers := []models.Transfer{
		{
			FromStopID:      "B1",
			ToStopID:        "B2",
			TransferType:    notPossible,
			MinTransferTime: nil,
		},
	}
	idx := Build(loc, window, stops, transfers, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	assert.Empty(
		t,
		journeys,
		"an explicit transfer_type=3 must block the default footpath",
	)
}

// TestSearchJourneys_MergesConsecutiveConnectionsIntoOneLeg rides a single
// train past an intermediate stop, so reconstruct merges two elementary
// connections into one Leg — the branch that carries the alight stop's
// display name onto the already-started leg.
func TestSearchJourneys_MergesConsecutiveConnectionsIntoOneLeg(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Charlie", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+600, 8*3600+600, 0, 0),
			mkST(3, "C1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	idx := Build(loc, window, baseStops(), nil, instances, flattenPatterns(patterns))

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	require.Len(t, journeys[0].Legs, 1)
	leg := journeys[0].Legs[0]
	assert.Equal(t, "A1", leg.BoardStopID)
	assert.Equal(t, "Alpha", leg.BoardStopName)
	assert.Equal(t, "C1", leg.AlightStopID)
	assert.Equal(t, "Charlie", leg.AlightStopName)
}

// TestMinTransferSeconds pins the connection-makeability lookup the live
// journey page uses (issue #1395): explicit transfers.txt footpath wins, an
// unknown pair falls back to the never-zero default, and the same stop is a
// free change.
func TestMinTransferSeconds(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 0)),
	}
	patterns := map[string][]models.StopTime{
		"t1": {
			mkST(1, "A1", 8*3600, 8*3600, 0, 0),
			mkST(2, "B1", 8*3600+1200, 8*3600+1200, 0, 0),
		},
	}
	mtt := 300
	transfers := []models.Transfer{
		{
			FromStopID: "B1", ToStopID: "B2",
			TransferType: 2, MinTransferTime: &mtt,
		},
	}
	idx := Build(
		loc,
		window,
		baseStops(),
		transfers,
		instances,
		flattenPatterns(patterns),
	)

	assert.Equal(t, 0, idx.MinTransferSeconds("B1", "B1"))
	assert.Equal(t, 300, idx.MinTransferSeconds("B1", "B2"))
	assert.Equal(t, defaultMinTransferSeconds, idx.MinTransferSeconds("A1", "C1"))
	assert.Equal(t, defaultMinTransferSeconds, idx.MinTransferSeconds("nope", "B2"))
}

// TestBuild_GroupsUnorderedStopTimes pins the ordering guarantee Build now
// owns: stop_times arriving out of stop_sequence order still build a
// correctly-ordered pattern (previously the caller's SQL ORDER BY was load-
// bearing and untested here).
func TestBuild_GroupsUnorderedStopTimes(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	instances := []models.ActiveTrip{
		mkTrip("t1", "100", "Bravo", day(window, 0)),
	}
	// deliberately reversed.
	stopTimes := []models.StopTime{
		{
			TripID:           "t1",
			StopSequence:     2,
			StopID:           "B1",
			ArrivalSeconds:   8*3600 + 1200,
			DepartureSeconds: 8*3600 + 1200,
			PickupType:       0,
			DropOffType:      0,
		},
		{
			TripID:           "t1",
			StopSequence:     1,
			StopID:           "A1",
			ArrivalSeconds:   8 * 3600,
			DepartureSeconds: 8 * 3600,
			PickupType:       0,
			DropOffType:      0,
		},
	}
	idx := Build(loc, window, baseStops(), nil, instances, stopTimes)

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	assert.Equal(t, "A1", journeys[0].Legs[0].BoardStopID)
	assert.Equal(t, "B1", journeys[0].Legs[0].AlightStopID)
}
