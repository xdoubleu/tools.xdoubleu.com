package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

const (
	brusselsNorth = "008812005"
	liege         = "008841004"
)

func fixtureServices(
	t *testing.T,
) (*services.StationsService, *services.JourneyService) {
	t.Helper()
	client := mocks.NewFixtureClient(t)
	stations := services.NewStationsService(client)
	return stations, services.NewJourneyService(client, stations)
}

func TestStationsService_SearchesEveryLanguage(t *testing.T) {
	stations, _ := fixtureServices(t)
	ctx := context.Background()

	for _, q := range []string{"brussel-noord", "Bruxelles-Nord", "BRUSSELS-NORTH"} {
		got, err := stations.SearchStations(ctx, q)
		require.NoError(t, err)
		require.NotEmpty(t, got, q)
		assert.Equal(t, brusselsNorth, got[0].StopID)
	}

	got, err := stations.SearchStations(ctx, "brussel-noord")
	require.NoError(t, err)
	assert.Equal(t, "Brussel-Noord", got[0].NameNL)
	assert.Equal(t, "Bruxelles-Nord", got[0].NameFR)
	assert.Equal(t, "Brussels-North", got[0].NameEN)
	assert.Equal(t, "Bruxelles-Nord / Brussel-Noord / Brussels-North", got[0].DisplayName)

	all, err := stations.SearchStations(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 20, "an empty query returns a capped alphabetical page")
}

func TestStationsService_ByID(t *testing.T) {
	stations, _ := fixtureServices(t)
	st, ok, err := stations.ByID(context.Background(), liege)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, st.DisplayName, "Liège")

	_, ok, err = stations.ByID(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStationsService_ServesStaleListWhenIRailFails(t *testing.T) {
	failing := false
	//nolint:exhaustruct //only the fields under test are set
	client := &mocks.StubClient{
		StationsFn: func(string) ([]irail.Station, error) {
			if failing {
				return nil, errors.New("down")
			}
			return []irail.Station{{ID: "1", Name: "A"}}, nil
		},
	}
	svc := services.NewStationsService(client)
	_, err := svc.SearchStations(context.Background(), "")
	require.NoError(t, err)

	failing = true
	got, err := svc.SearchStations(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	cold := services.NewStationsService(client)
	_, err = cold.SearchStations(context.Background(), "")
	require.Error(t, err, "no list to fall back on")
}

func TestJourneyService_SearchJourneys(t *testing.T) {
	_, journeys := fixtureServices(t)
	ctx := context.Background()

	got, err := journeys.SearchJourneys(
		ctx, brusselsNorth, liege, time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), false,
	)
	require.NoError(t, err)
	require.Len(t, got, 6)

	var withTransfer int
	for i, j := range got {
		require.NotEmpty(t, j.Legs)
		assert.Equal(t, brusselsNorth, j.Legs[0].BoardStopID)
		assert.Equal(t, liege, j.Legs[len(j.Legs)-1].AlightStopID)
		assert.Equal(t, len(j.Legs)-1, j.Transfers)
		assert.Contains(t, j.Legs[0].BoardStopName, "Brussel-Noord")
		if i > 0 {
			assert.False(
				t,
				j.DepartureTime.Before(got[i-1].DepartureTime),
				"ordered by departure",
			)
		}
		if j.Transfers > 0 {
			withTransfer++
		}

		refs, decodeErr := services.DecodeJourneyID(j.JourneyID)
		require.NoError(t, decodeErr)
		require.Len(t, refs, len(j.Legs))
		assert.Equal(
			t,
			"BE.NMBS."+j.Legs[0].RouteShortName+j.Legs[0].TripShortName,
			refs[0].VehicleID,
		)
	}
	assert.Positive(t, withTransfer, "the fixture holds journeys with a change")
}

func TestJourneyService_UnknownStation(t *testing.T) {
	_, journeys := fixtureServices(t)
	_, err := journeys.SearchJourneys(
		context.Background(),
		"nope",
		liege,
		time.Now(),
		false,
	)
	require.ErrorIs(t, err, services.ErrUnknownStation)
	_, err = journeys.SearchJourneys(
		context.Background(),
		liege,
		"nope",
		time.Now(),
		false,
	)
	require.ErrorIs(t, err, services.ErrUnknownStation)
}

// ic530 is the recorded run: Liège-Guillemins, Leuven, Brussels-North in order.
func ic530Journey(t *testing.T) string {
	t.Helper()
	day := time.Date(2026, 10, 23, 6, 0, 0, 0, time.UTC)
	return services.EncodeJourneyID([]services.LegRef{{
		VehicleID: "BE.NMBS.IC530", BoardStopID: liege, BoardTime: day,
		AlightStopID: brusselsNorth, AlightTime: day.Add(time.Hour),
	}})
}

func TestJourneyDetail_FromRecordedTrain(t *testing.T) {
	client := mocks.NewFixtureClient(t)
	stations := services.NewStationsService(client)
	detail := services.NewJourneyDetailService(client, stations, nil)

	got, err := detail.GetJourneyDetail(context.Background(), ic530Journey(t))
	require.NoError(t, err)
	require.Len(t, got.Legs, 1)

	leg := got.Legs[0]
	assert.Equal(t, "530", leg.TripShortName)
	assert.Equal(t, "IC", leg.RouteShortName)
	assert.False(t, leg.Cancelled)
	require.Len(t, leg.Stops, 3, "only the boarded stretch")
	assert.True(t, leg.Stops[0].IsBoardStop)
	assert.True(t, leg.Stops[2].IsAlightStop)
	assert.Equal(t, liege, leg.Stops[0].StopID)
	assert.Contains(t, leg.Stops[0].StopName, "Liège")
	assert.Equal(t, models.DelayDelayed, leg.Stops[0].State, "9 min late at departure")
	require.NotNil(t, leg.Stops[0].DepartureDelay)
	assert.Equal(t, 540, *leg.Stops[0].DepartureDelay)
	assert.Nil(t, got.Alternative, "a late train alone does not break the journey")
}

func vehicleOf(calls ...irail.Call) *mocks.StubClient {
	//nolint:exhaustruct //only the fields under test are set
	return &mocks.StubClient{
		VehicleFn: func(string) (*irail.Vehicle, error) {
			return &irail.Vehicle{ID: "BE.NMBS.IC1", Type: "IC", Num: "1", Calls: calls}, nil
		},
	}
}

func oneLegID() string {
	t0 := time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)
	return services.EncodeJourneyID([]services.LegRef{{
		VehicleID: "BE.NMBS.IC1", BoardStopID: "A", BoardTime: t0,
		AlightStopID: "C", AlightTime: t0.Add(time.Hour),
	}})
}

func TestJourneyDetail_StopStates(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)
	call := func(id string, delay int, canceled bool) irail.Call {
		//nolint:exhaustruct //only the fields under test are set
		return irail.Call{
			StationID: id, Name: id, ScheduledArrival: t0, ScheduledDeparture: t0,
			ArrivalDelay: delay, DepartureDelay: delay,
			ArrivalCanceled: canceled, DepartureCanceled: canceled,
		}
	}
	client := vehicleOf(call("A", 0, false), call("B", 0, true), call("C", 59, false))
	detail := services.NewJourneyDetailService(
		client,
		services.NewStationsService(client),
		nil,
	)

	got, err := detail.GetJourneyDetail(context.Background(), oneLegID())
	require.NoError(t, err)
	stops := got.Legs[0].Stops
	require.Len(t, stops, 3)
	assert.Equal(t, models.DelayOnTime, stops[0].State)
	assert.Equal(
		t,
		models.DelaySkipped,
		stops[1].State,
		"a cancelled stop on a running train is skipped",
	)
	assert.Equal(t, models.DelayOnTime, stops[2].State, "under a minute is on time")
	assert.False(t, got.Legs[0].Cancelled)
}

func TestJourneyDetail_WholeTrainCancelled(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)
	cancelled := func(id string) irail.Call {
		//nolint:exhaustruct //only the fields under test are set
		return irail.Call{
			StationID: id, ScheduledArrival: t0, ScheduledDeparture: t0,
			ArrivalCanceled: true, DepartureCanceled: true,
		}
	}
	client := vehicleOf(cancelled("A"), cancelled("B"), cancelled("C"))
	detail := services.NewJourneyDetailService(
		client,
		services.NewStationsService(client),
		nil,
	)

	got, err := detail.GetJourneyDetail(context.Background(), oneLegID())
	require.NoError(t, err)
	assert.True(t, got.Legs[0].Cancelled)
	for _, s := range got.Legs[0].Stops {
		assert.Equal(t, models.DelayCancelled, s.State)
	}
}

func TestJourneyDetail_UnknownTrainRendersEmptyLeg(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	client := &mocks.StubClient{} // every vehicle is not found
	detail := services.NewJourneyDetailService(
		client,
		services.NewStationsService(client),
		nil,
	)

	got, err := detail.GetJourneyDetail(context.Background(), oneLegID())
	require.NoError(t, err)
	assert.Equal(t, "IC", got.Legs[0].RouteShortName)
	assert.Equal(t, "1", got.Legs[0].TripShortName)
	assert.Empty(t, got.Legs[0].Stops)
}

func TestJourneyDetail_IRailFailureIsAnError(t *testing.T) {
	//nolint:exhaustruct //only the fields under test are set
	client := &mocks.StubClient{
		VehicleFn: func(string) (*irail.Vehicle, error) { return nil, irail.ErrRateLimited },
	}
	detail := services.NewJourneyDetailService(
		client,
		services.NewStationsService(client),
		nil,
	)

	_, err := detail.GetJourneyDetail(context.Background(), oneLegID())
	require.ErrorIs(t, err, irail.ErrRateLimited)

	_, err = detail.GetJourneyDetail(context.Background(), "garbage")
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
}

func TestJourneyService_MinTransferSeconds(t *testing.T) {
	_, journeys := fixtureServices(t)
	assert.Equal(t, 180, journeys.MinTransferSeconds(brusselsNorth, liege))
}
