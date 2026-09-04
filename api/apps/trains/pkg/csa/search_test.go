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
)

var loc = time.UTC //nolint:gochecknoglobals //test fixture timezone

// stations sharing a parent — mirrors the real feed's "S"+UIC station /
// child-platform shape (issue #1389/#1391).
func baseStops() []StopInput {
	return []StopInput{
		{ID: "SA", Name: "Alpha", ParentStation: "", PlatformCode: "", LocationType: 1},
		{
			ID:            "A1",
			Name:          "Alpha",
			ParentStation: "SA",
			PlatformCode:  "1",
			LocationType:  0,
		},
		{ID: "SB", Name: "Bravo", ParentStation: "", PlatformCode: "", LocationType: 1},
		{
			ID:            "B1",
			Name:          "Bravo",
			ParentStation: "SB",
			PlatformCode:  "1",
			LocationType:  0,
		},
		{
			ID:            "B2",
			Name:          "Bravo",
			ParentStation: "SB",
			PlatformCode:  "2",
			LocationType:  0,
		},
		{
			ID:            "SC",
			Name:          "Charlie",
			ParentStation: "",
			PlatformCode:  "",
			LocationType:  1,
		},
		{
			ID:            "C1",
			Name:          "Charlie",
			ParentStation: "SC",
			PlatformCode:  "1",
			LocationType:  0,
		},
	}
}

func day(base time.Time, n int) time.Time { return base.AddDate(0, 0, n) }

func TestSearchJourneys_Direct(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []TripInstanceInput{
		{
			TripID: "t1", ShortName: "100", RouteShortName: "IC", Headsign: "Bravo",
			Date: day(window, 0),
		},
	}
	patterns := map[string][]StopTimeInput{
		"t1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	require.NotEmpty(t, journeys)
	j := journeys[0]
	assert.Len(t, j.Legs, 1)
	assert.Equal(t, 0, j.Transfers)
	assert.Equal(t, "100", j.Legs[0].TripShortName)
	assert.Equal(t, "A1", j.Legs[0].BoardStopID)
	assert.Equal(t, "B1", j.Legs[0].AlightStopID)
	assert.Equal(t, day(window, 0).Add(8*time.Hour+20*time.Minute), j.ArrivalTime)
}

func TestSearchJourneys_RequiresTransfer(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []TripInstanceInput{
		{
			TripID:         "leg1",
			ShortName:      "100",
			RouteShortName: "IC",
			Headsign:       "Bravo",
			Date:           day(window, 0),
		},
		{
			TripID:         "leg2",
			ShortName:      "200",
			RouteShortName: "IC",
			Headsign:       "Charlie",
			Date:           day(window, 0),
		},
	}
	patterns := map[string][]StopTimeInput{
		"leg1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
		// same-station change B1 -> B2 must cost the default transfer time,
		// never a free 0-minute move (issue #1391's core trap): the gap
		// here is 300s, comfortably above defaultMinTransferSeconds.
		"leg2": {
			{
				StopSequence:     1,
				StopID:           "B2",
				ArrivalSeconds:   8*3600 + 1500,
				DepartureSeconds: 8*3600 + 1500,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "C1",
				ArrivalSeconds:   8*3600 + 2700,
				DepartureSeconds: 8*3600 + 2700,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

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
	patterns["leg2"][0] = StopTimeInput{
		StopSequence: 1, StopID: "B2",
		ArrivalSeconds:   8*3600 + 1200 + defaultMinTransferSeconds - 1,
		DepartureSeconds: 8*3600 + 1200 + defaultMinTransferSeconds - 1,
		PickupType:       0,
		DropOffType:      0,
	}
	idx2 := Build(loc, window, stops, nil, instances, patterns)
	journeys2, err := idx2.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	assert.Empty(t, journeys2, "should not offer a sub-minimum same-station transfer")
}

func TestSearchJourneys_AfterMidnightCrossesServiceDay(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := baseStops()
	instances := []TripInstanceInput{
		{
			TripID: "night", ShortName: "900", RouteShortName: "IC", Headsign: "Bravo",
			Date: day(window, 0),
		},
	}
	// departs 23:50 day 0, arrives 25:10 (day 0's GTFS clock) == 01:10 day 1.
	patterns := map[string][]StopTimeInput{
		"night": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   23*3600 + 3000,
				DepartureSeconds: 23*3600 + 3000,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   25*3600 + 600,
				DepartureSeconds: 25*3600 + 600,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

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
	instances := []TripInstanceInput{
		{
			TripID:         "t1",
			ShortName:      "100",
			RouteShortName: "IC",
			Headsign:       "Bravo",
			Date:           day(window, 5),
		},
	}
	patterns := map[string][]StopTimeInput{
		"t1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

	when := time.Date(2026, 10, 1, 7, 0, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SB", when, false)
	require.NoError(t, err)
	assert.Empty(t, journeys)
}

func TestSearchJourneys_NonBoardingCallNotOfferedAsBoardingPoint(t *testing.T) {
	window := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	stops := append(
		baseStops(),
		StopInput{
			ID:            "M1",
			Name:          "Midway",
			ParentStation: "",
			PlatformCode:  "",
			LocationType:  0,
		},
	)
	instances := []TripInstanceInput{
		{
			TripID:         "t1",
			ShortName:      "100",
			RouteShortName: "IC",
			Headsign:       "Bravo",
			Date:           day(window, 0),
		},
	}
	patterns := map[string][]StopTimeInput{
		"t1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			// technical pass-through: neither boardable nor alightable.
			{
				StopSequence: 2, StopID: "M1",
				ArrivalSeconds: 8*3600 + 600, DepartureSeconds: 8*3600 + 600,
				PickupType: 1, DropOffType: 1,
			},
			{
				StopSequence:     3,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

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
	instances := []TripInstanceInput{
		{
			TripID:         "t1",
			ShortName:      "100",
			RouteShortName: "IC",
			Headsign:       "Bravo",
			Date:           day(window, 0),
		},
	}
	patterns := map[string][]StopTimeInput{
		"t1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	idx := Build(loc, window, stops, nil, instances, patterns)

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
	instances := []TripInstanceInput{
		{
			TripID:         "leg1",
			ShortName:      "100",
			RouteShortName: "IC",
			Headsign:       "Bravo",
			Date:           day(window, 0),
		},
		{
			TripID:         "leg2",
			ShortName:      "200",
			RouteShortName: "IC",
			Headsign:       "Charlie",
			Date:           day(window, 0),
		},
	}
	patterns := map[string][]StopTimeInput{
		"leg1": {
			{
				StopSequence:     1,
				StopID:           "A1",
				ArrivalSeconds:   8 * 3600,
				DepartureSeconds: 8 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "B1",
				ArrivalSeconds:   8*3600 + 1200,
				DepartureSeconds: 8*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
		"leg2": {
			{
				StopSequence:     1,
				StopID:           "B2",
				ArrivalSeconds:   20 * 3600,
				DepartureSeconds: 20 * 3600,
				PickupType:       0,
				DropOffType:      0,
			},
			{
				StopSequence:     2,
				StopID:           "C1",
				ArrivalSeconds:   20*3600 + 1200,
				DepartureSeconds: 20*3600 + 1200,
				PickupType:       0,
				DropOffType:      0,
			},
		},
	}
	const notPossible = 3
	transfers := []TransferInput{
		{
			FromStopID:      "B1",
			ToStopID:        "B2",
			TransferType:    notPossible,
			MinTransferTime: nil,
		},
	}
	idx := Build(loc, window, stops, transfers, instances, patterns)

	when := time.Date(2026, 10, 1, 7, 55, 0, 0, loc)
	journeys, err := idx.SearchJourneys("SA", "SC", when, false)
	require.NoError(t, err)
	assert.Empty(
		t,
		journeys,
		"an explicit transfer_type=3 must block the default footpath",
	)
}
