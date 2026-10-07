package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// overlayPattern: Leuven, Brussel-Noord, Brussel-Centraal, Brussel-Zuid at
// 08:00, 08:15, 08:20, 08:25, each stopping one minute.
func overlayPattern() []models.StopTime {
	const h = 8 * 3600
	stop := func(seq int, id string, minute int) models.StopTime {
		//nolint:exhaustruct //trip id and pickup/drop-off types are irrelevant
		return models.StopTime{
			StopSequence:     seq,
			StopID:           id,
			ArrivalSeconds:   h + minute*60,
			DepartureSeconds: h + (minute+1)*60,
		}
	}
	return []models.StopTime{
		stop(1, "gs:nmbssncb:8833001_2", 0),
		stop(2, "gs:nmbssncb:8812005_5", 15),
		stop(3, "gs:nmbssncb:8813003_3", 20),
		stop(4, "gs:nmbssncb:8814001_7", 25),
	}
}

func scheduledCall(stopID string, seq, delay int) models.StopCall {
	return models.StopCall{
		StopID:         stopID,
		StopSequence:   seq,
		State:          stateForDelays(&delay, &delay),
		ArrivalDelay:   &delay,
		DepartureDelay: &delay,
		ArrivalTime:    nil,
		DepartureTime:  nil,
	}
}

func statusCall(stopID string, seq int, state models.DelayState) models.StopCall {
	//nolint:exhaustruct //a status-only call carries no delay or time
	return models.StopCall{StopID: stopID, StopSequence: seq, State: state}
}

func overlayDate() time.Time {
	return time.Date(2026, 10, 7, 0, 0, 0, 0, brusselsLoc)
}

func TestOverlayCalls_NoCallsIsAllUnknown(t *testing.T) {
	out := overlayCalls(overlayPattern(), nil, overlayDate())
	require.Len(t, out, 4)
	for _, c := range out {
		assert.Equal(t, models.DelayUnknown, c.State)
	}
}

// TestOverlayCalls_PlacesByStationWithoutStopSequence: GTFS-RT may omit
// stop_sequence and name a different platform than the timetable.
func TestOverlayCalls_PlacesByStationWithoutStopSequence(t *testing.T) {
	calls := []models.StopCall{
		scheduledCall("gs:nmbssncb:8812005", 0, 120),
		statusCall("gs:nmbssncb:8813003_4", 0, models.DelayUnknown),
	}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	assert.Equal(t, models.DelayUnknown, out[0].State, "nothing before Noord")
	assert.Equal(t, models.DelayDelayed, out[1].State)
	require.NotNil(t, out[1].ArrivalDelay)
	assert.Equal(t, 120, *out[1].ArrivalDelay)
	assert.Equal(t, models.DelayUnknown, out[2].State)
}

// TestOverlayCalls_StationWinsOverMismatchedStopSequence: the realtime trip's
// numbering can differ from the static trip journey detail resolved.
func TestOverlayCalls_StationWinsOverMismatchedStopSequence(t *testing.T) {
	calls := []models.StopCall{
		scheduledCall("gs:nmbssncb:8812005_5", 4, 60),
		scheduledCall("gs:nmbssncb:8814001_7", 9, 180),
	}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	require.NotNil(t, out[1].ArrivalDelay)
	assert.Equal(t, 60, *out[1].ArrivalDelay, "sequence 4 is Zuid, but the call is Noord")
	require.NotNil(t, out[3].ArrivalDelay)
	assert.Equal(t, 180, *out[3].ArrivalDelay)
}

// TestOverlayCalls_StopSequenceAloneStillPlaces: a call with no stop_id is
// placed by its sequence.
func TestOverlayCalls_StopSequenceAloneStillPlaces(t *testing.T) {
	calls := []models.StopCall{scheduledCall("", 3, 240)}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	require.NotNil(t, out[2].ArrivalDelay)
	assert.Equal(t, 240, *out[2].ArrivalDelay)
	assert.Equal(t, models.DelayUnknown, out[1].State)
}

func TestOverlayCalls_UnplaceableCallsAreDropped(t *testing.T) {
	calls := []models.StopCall{
		scheduledCall("", 99, 60),
		scheduledCall("gs:nmbssncb:8892007_1", 0, 60),
	}
	out := overlayCalls(overlayPattern(), calls, overlayDate())
	for _, c := range out {
		assert.Equal(t, models.DelayUnknown, c.State)
	}
}

// TestOverlayCalls_PropagatesDelayToStopsWithoutUpdate follows GTFS-RT: a stop
// with no update inherits the last published delay.
func TestOverlayCalls_PropagatesDelayToStopsWithoutUpdate(t *testing.T) {
	calls := []models.StopCall{scheduledCall("gs:nmbssncb:8833001_2", 1, 300)}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	for i := range out {
		assert.Equal(t, models.DelayDelayed, out[i].State, "stop %d", i)
		require.NotNil(t, out[i].ArrivalDelay)
		assert.Equal(t, 300, *out[i].ArrivalDelay)
	}
	assert.Equal(t, "gs:nmbssncb:8814001_7", out[3].StopID)
}

func TestOverlayCalls_PropagatesOnTime(t *testing.T) {
	calls := []models.StopCall{scheduledCall("gs:nmbssncb:8812005_5", 2, 0)}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	assert.Equal(t, models.DelayUnknown, out[0].State, "never propagated backwards")
	assert.Equal(t, models.DelayOnTime, out[3].State)
}

func TestOverlayCalls_NoDataStopsPropagation(t *testing.T) {
	calls := []models.StopCall{
		scheduledCall("gs:nmbssncb:8833001_2", 1, 300),
		statusCall("gs:nmbssncb:8812005_5", 2, models.DelayUnknown),
	}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	assert.Equal(t, models.DelayDelayed, out[0].State)
	assert.Equal(t, models.DelayUnknown, out[1].State)
	assert.Equal(t, models.DelayUnknown, out[2].State)
	assert.Equal(t, models.DelayUnknown, out[3].State)
}

func TestOverlayCalls_SkippedStopKeepsCarriedDelay(t *testing.T) {
	calls := []models.StopCall{
		scheduledCall("gs:nmbssncb:8833001_2", 1, 120),
		statusCall("gs:nmbssncb:8812005_5", 2, models.DelaySkipped),
	}
	out := overlayCalls(overlayPattern(), calls, overlayDate())

	assert.Equal(t, models.DelaySkipped, out[1].State)
	require.NotNil(t, out[2].ArrivalDelay)
	assert.Equal(t, 120, *out[2].ArrivalDelay)
}

// TestOverlayCalls_DerivesDelayFromAbsoluteTimes: a call publishing only
// times is compared against the timetable.
func TestOverlayCalls_DerivesDelayFromAbsoluteTimes(t *testing.T) {
	date := overlayDate()
	arr := date.Add((8*3600 + 15*60 + 90) * time.Second)
	dep := date.Add((8*3600 + 16*60 + 90) * time.Second)
	//nolint:exhaustruct //a times-only SCHEDULED call, as decoded
	call := models.StopCall{
		StopID:        "gs:nmbssncb:8812005_5",
		StopSequence:  2,
		State:         models.DelayUnknown,
		ArrivalTime:   &arr,
		DepartureTime: &dep,
	}
	out := overlayCalls(overlayPattern(), []models.StopCall{call}, date)

	assert.Equal(t, models.DelayDelayed, out[1].State)
	require.NotNil(t, out[1].ArrivalDelay)
	assert.Equal(t, 90, *out[1].ArrivalDelay)
	require.NotNil(t, out[2].ArrivalDelay, "the derived delay propagates")
	assert.Equal(t, 90, *out[2].ArrivalDelay)
}

func TestStationKey(t *testing.T) {
	assert.Equal(t, "8814001", stationKey("gs:nmbssncb:8814001_7"))
	assert.Equal(t, "8814001", stationKey("gs:nmbssncb:S8814001"))
	assert.Equal(t, "nmbssncb:s-anversdam", stationKey("nmbssncb:s-anversdam"))
}
