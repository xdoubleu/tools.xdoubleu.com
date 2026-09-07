package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

func TestLegRange(t *testing.T) {
	//nolint:exhaustruct //only StopID is relevant to legRange
	pattern := []models.StopTime{
		{StopID: "X"}, {StopID: "A"}, {StopID: "M"}, {StopID: "B"}, {StopID: "Y"},
	}

	boardIdx, alightIdx := legRange(pattern, "A", "B")
	assert.Equal(t, 1, boardIdx)
	assert.Equal(t, 3, alightIdx)

	boardIdx, alightIdx = legRange(pattern, "does-not-exist", "B")
	assert.Equal(t, -1, boardIdx)
	assert.Equal(t, 3, alightIdx)
}

func stopCall(arrivalDelay, departureDelay *int) models.StopCall {
	//nolint:exhaustruct //only the delay fields under test here are set
	return models.StopCall{ArrivalDelay: arrivalDelay, DepartureDelay: departureDelay}
}

func TestExceedsThreshold(t *testing.T) {
	small := 30
	large := 120
	negLarge := -120
	assert.False(t, exceedsThreshold(stopCall(&small, nil)))
	assert.True(t, exceedsThreshold(stopCall(&large, nil)))
	assert.True(t, exceedsThreshold(stopCall(nil, &negLarge)))
	assert.False(t, exceedsThreshold(stopCall(nil, nil)))
}

func TestAbsInt(t *testing.T) {
	assert.Equal(t, 5, absInt(5))
	assert.Equal(t, 5, absInt(-5))
	assert.Equal(t, 0, absInt(0))
}

func TestApplyLiveState(t *testing.T) {
	t.Run(
		"cancelled trip forces DelayCancelled regardless of call",
		func(t *testing.T) {
			//nolint:exhaustruct //StopID/StopSequence not relevant to this check
			d := models.StopDetail{}
			call := models.StopCall{ //nolint:exhaustruct //only State is relevant here
				State: models.DelayOnTime,
			}
			applyLiveState(&d, call, true)
			assert.Equal(t, models.DelayCancelled, d.State)
		},
	)

	t.Run("no call decoded leaves DelayUnknown", func(t *testing.T) {
		//nolint:exhaustruct //zero-value target under test
		d := models.StopDetail{}
		//nolint:exhaustruct //zero-value call: nothing was decoded for this stop
		applyLiveState(&d, models.StopCall{}, false)
		assert.Equal(t, models.DelayUnknown, d.State)
	})

	t.Run(
		"delay under threshold renders as on-time, not delayed by 0",
		func(t *testing.T) {
			small := 5
			//nolint:exhaustruct //zero-value target under test
			d := models.StopDetail{}
			applyLiveState(&d, delayedCall(small), false)
			assert.Equal(t, models.DelayOnTime, d.State)
		},
	)

	t.Run("delay over threshold stays delayed", func(t *testing.T) {
		large := 300
		//nolint:exhaustruct //zero-value target under test
		d := models.StopDetail{}
		applyLiveState(&d, delayedCall(large), false)
		assert.Equal(t, models.DelayDelayed, d.State)
		assert.Equal(t, &large, d.ArrivalDelay)
	})

	t.Run("skipped stop is never reported as on-time", func(t *testing.T) {
		//nolint:exhaustruct //zero-value target under test
		d := models.StopDetail{}
		call := models.StopCall{ //nolint:exhaustruct //only State is relevant here
			State: models.DelaySkipped,
		}
		applyLiveState(&d, call, false)
		assert.Equal(t, models.DelaySkipped, d.State)
	})
}

func delayedCall(arrivalDelaySeconds int) models.StopCall {
	//nolint:exhaustruct //only State/ArrivalDelay are relevant to applyLiveState here
	return models.StopCall{
		State:        models.DelayDelayed,
		ArrivalDelay: &arrivalDelaySeconds,
	}
}

func alert(id string, tripIDs, routeIDs, stopIDs []string) models.Alert {
	//nolint:exhaustruct //only the id and one informed-entity list are relevant here
	return models.Alert{
		ID:               id,
		InformedTripIDs:  tripIDs,
		InformedRouteIDs: routeIDs,
		InformedStopIDs:  stopIDs,
	}
}

func TestAlertsForLeg(t *testing.T) {
	//nolint:exhaustruct //only TripID/RouteID are relevant to alertsForLeg here
	active := &repositories.ActiveTrip{TripID: "t1", RouteID: "r1"}
	alerts := []models.Alert{
		alert("a1", []string{"t1"}, nil, nil),
		alert("a2", nil, []string{"r1"}, nil),
		alert("a3", nil, nil, []string{"S1"}),
		alert("a4", nil, nil, []string{"other"}),
	}

	out := alertsForLeg(alerts, active, []string{"S1", "S2"})
	a := assert.New(t)
	a.Len(out, 3)
	ids := []string{out[0].ID, out[1].ID, out[2].ID}
	a.Contains(ids, "a1")
	a.Contains(ids, "a2")
	a.Contains(ids, "a3")
}

func TestServiceDateOf(t *testing.T) {
	t.Setenv("TZ", "UTC")
	loc, _ := time.LoadLocation("Europe/Brussels")
	when := time.Date(2026, 9, 7, 23, 30, 0, 0, loc)
	date := serviceDateOf(when)
	assert.Equal(t, 2026, date.Year())
	assert.Equal(t, time.September, date.Month())
	assert.Equal(t, 7, date.Day())
	assert.Equal(t, 0, date.Hour())
}

func TestBuildStopDetails_NoLiveDataIsNeverOnTime(t *testing.T) {
	//nolint:exhaustruct //only fields relevant to this stop's position are set
	pattern := []models.StopTime{
		{StopID: "A", StopSequence: 1, ArrivalSeconds: 0, DepartureSeconds: 60},
	}
	//nolint:exhaustruct //only board/alight identification is relevant here
	ref := LegRef{TripShortName: "IC1", BoardStopID: "A", AlightStopID: "B"}

	details := buildStopDetails(
		pattern,
		map[string]models.Stop{},
		nil,
		false,
		time.Now(),
		ref,
	)
	a := assert.New(t)
	a.Len(details, 1)
	a.Equal(models.DelayUnknown, details[0].State)
	a.True(details[0].IsBoardStop)
	a.False(details[0].IsAlightStop)
}
