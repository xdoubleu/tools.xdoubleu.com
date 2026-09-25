package trains_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
)

func newTrainsTestClient(t *testing.T) trainsv1connect.TrainServiceClient {
	t.Helper()
	ts := httptest.NewServer(getTrainsRoutes())
	t.Cleanup(ts.Close)
	return trainsv1connect.NewTrainServiceClient(
		http.DefaultClient, ts.URL, connect.WithHTTPGet(),
	)
}

func getTrainsRoutes() http.Handler {
	mux := http.NewServeMux()
	testApp.Routes(testApp.GetName(), mux)
	return mux
}

// TestSearchJourneys_Handler_UnknownStop covers csa.ErrUnknownStop ->
// CodeNotFound over real HTTP.
func TestSearchJourneys_Handler_UnknownStop(t *testing.T) {
	ctx := context.Background()
	// Warm the router, else SearchJourneys returns CodeUnavailable.
	_, err := testApp.Services.Journey.RefreshWindow(
		ctx, time.Now().UTC().Truncate(24*time.Hour),
	)
	require.NoError(t, err)

	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.SearchJourneysRequest{
		OriginStopId:      "does-not-exist",
		DestinationStopId: "also-not-real",
	})

	_, err = client.SearchJourneys(ctx, req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

// TestSearchJourneys_Handler_InvalidTime covers a non-RFC3339 time.
func TestSearchJourneys_Handler_InvalidTime(t *testing.T) {
	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.SearchJourneysRequest{
		OriginStopId:      "SA",
		DestinationStopId: "SB",
		Time:              "not-a-timestamp",
	})

	_, err := client.SearchJourneys(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// TestSearchJourneys_Handler_Success checks protoJourney mapping end-to-end.
func TestSearchJourneys_Handler_Success(t *testing.T) {
	ctx := context.Background()
	windowStart := time.Now().UTC().Truncate(24 * time.Hour)

	require.NoError(
		t,
		testApp.Repositories.Feed.ImportFeed(ctx, journeyFeed(windowStart)),
	)
	_, refreshErr := testApp.Services.Journey.RefreshWindow(ctx, windowStart)
	require.NoError(t, refreshErr)

	client := newTrainsTestClient(t)
	// SA->SC needs the same-station transfer at Bravo from journey_test.go.
	when := windowStart.Add(8*time.Hour + 55*time.Minute)
	req := connect.NewRequest(&trainsv1.SearchJourneysRequest{
		OriginStopId:      "SA",
		DestinationStopId: "SC",
		Time:              when.Format(time.RFC3339),
	})

	resp, err := client.SearchJourneys(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetJourneys())
	// SearchJourneys returns a window of journeys; don't assume position.
	j := findJourneyByFirstLeg(t, resp.Msg.GetJourneys(), "200")
	require.Len(t, j.GetLegs(), 2)
	assert.Equal(t, int32(1), j.GetTransfers())
	assert.Equal(t, "200", j.GetLegs()[0].GetTripShortName())
	assert.Equal(t, "B1", j.GetLegs()[0].GetAlightStopId())
	assert.Equal(t, "300", j.GetLegs()[1].GetTripShortName())
	assert.Equal(t, "B2", j.GetLegs()[1].GetBoardStopId())
	assert.NotEmpty(t, j.GetLegs()[0].GetBoardTime())
	assert.NotEmpty(t, j.GetLegs()[1].GetAlightTime())
	assert.NotEmpty(t, j.GetDepartureTime())
	assert.NotEmpty(t, j.GetArrivalTime())
}

func findJourneyByFirstLeg(
	t *testing.T, journeys []*trainsv1.Journey, tripShortName string,
) *trainsv1.Journey {
	t.Helper()
	for _, j := range journeys {
		if len(j.GetLegs()) > 0 && j.GetLegs()[0].GetTripShortName() == tripShortName {
			return j
		}
	}
	require.Fail(t, "no journey found with first leg "+tripShortName)
	return nil
}
