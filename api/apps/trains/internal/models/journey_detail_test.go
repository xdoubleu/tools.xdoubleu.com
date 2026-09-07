package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// TestStopDetail_MarshalJSON pins the wire shape the journey websocket push
// and the GetJourneyDetail RPC must agree on (issue #1394) — camelCase
// fields, "unknown" never omitted or defaulted to "on_time".
func TestStopDetail_MarshalJSON(t *testing.T) {
	arr := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	dep := arr.Add(2 * time.Minute)
	delay := 180
	d := models.StopDetail{
		StopID:             "A1",
		StopName:           "Alpha",
		Platform:           "3",
		ScheduledArrival:   &arr,
		ScheduledDeparture: &dep,
		State:              models.DelayDelayed,
		ArrivalDelay:       &delay,
		DepartureDelay:     nil,
		IsBoardStop:        true,
		IsAlightStop:       false,
	}

	body, err := json.Marshal(d)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "A1", out["stopId"])
	assert.Equal(t, "Alpha", out["stopName"])
	assert.Equal(t, "3", out["platform"])
	assert.Equal(t, "2026-09-07T08:00:00Z", out["scheduledArrival"])
	assert.Equal(t, "delayed", out["status"])
	assert.Equal(t, float64(180), out["delaySeconds"])
	assert.Equal(t, true, out["isBoardStop"])
}

func TestStopDetail_MarshalJSON_UnknownStateAndNoScheduledTimes(t *testing.T) {
	//nolint:exhaustruct //zero-value StopDetail is exactly what's under test
	d := models.StopDetail{StopID: "A1", State: models.DelayUnknown}

	body, err := json.Marshal(d)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "unknown", out["status"])
	// omitempty on both scheduled fields — never emitted at all, never "".
	_, hasArrival := out["scheduledArrival"]
	assert.False(t, hasArrival)
	_, hasDeparture := out["scheduledDeparture"]
	assert.False(t, hasDeparture)
}

func TestStopDetail_MarshalJSON_DepartureDelayFallback(t *testing.T) {
	delay := -30
	//nolint:exhaustruct //only DepartureDelay is under test here
	d := models.StopDetail{
		StopID:         "A1",
		State:          models.DelayDelayed,
		DepartureDelay: &delay,
	}

	body, err := json.Marshal(d)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, float64(-30), out["delaySeconds"])
}

func TestLegDetail_MarshalJSON_NilSlicesBecomeEmptyArrays(t *testing.T) {
	//nolint:exhaustruct //Stops/Alerts intentionally nil, under test
	leg := models.LegDetail{TripShortName: "IC1"}

	body, err := json.Marshal(leg)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, []any{}, out["stops"])
	assert.Equal(t, []any{}, out["alerts"])
	assert.Equal(t, false, out["cancelled"])
}

func TestJourneyDetail_MarshalJSON(t *testing.T) {
	dep := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	arr := dep.Add(time.Hour)
	//nolint:exhaustruct //Legs intentionally nil, under test
	j := models.JourneyDetail{DepartureTime: dep, ArrivalTime: arr}

	body, err := json.Marshal(j)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, []any{}, out["legs"])
	assert.Equal(t, "2026-09-07T08:00:00Z", out["departureTime"])
	assert.Equal(t, "2026-09-07T09:00:00Z", out["arrivalTime"])
}

func TestAlert_MarshalJSON_OnlyExposesPassengerFacingFields(t *testing.T) {
	a := models.Alert{
		ID:               "a1",
		Cause:            "TECHNICAL_PROBLEM",
		Effect:           "DELAYED",
		HeaderText:       "Signal failure",
		DescriptionText:  "Delays expected",
		InformedTripIDs:  []string{"t1"},
		InformedRouteIDs: []string{"r1"},
		InformedStopIDs:  []string{"s1"},
	}

	body, err := json.Marshal(a)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "a1", out["id"])
	assert.Equal(t, "Signal failure", out["headerText"])
	assert.Equal(t, "Delays expected", out["descriptionText"])
	assert.NotContains(t, out, "cause")
	assert.NotContains(t, out, "effect")
	assert.NotContains(t, out, "informedTripIDs")
}
