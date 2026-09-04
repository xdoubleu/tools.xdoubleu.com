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

// TestSearchJourneys_Handler_UnknownStop exercises the handler end-to-end
// over real HTTP, covering the CodeNotFound mapping of csa.ErrUnknownStop
// (connect_journeys.go's mapError).
func TestSearchJourneys_Handler_UnknownStop(t *testing.T) {
	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.SearchJourneysRequest{
		OriginStopId:      "does-not-exist",
		DestinationStopId: "also-not-real",
	})

	_, err := client.SearchJourneys(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

// TestSearchJourneys_Handler_InvalidTime covers the CodeInvalidArgument
// branch when the request's time isn't valid RFC3339.
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

// TestSearchJourneys_Handler_Success drives the handler over real HTTP
// against the same fixture journey_test.go seeds, verifying the
// protoJourney field mapping (legs, times, transfer count) end-to-end.
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
	// SA->SC requires the same-station transfer at Bravo exercised by
	// journey_test.go's "journey requiring a transfer" subtest — picked
	// here (rather than the direct SA->SB trip) so board/alight stop IDs
	// are asserted against a scenario already proven unambiguous.
	when := windowStart.Add(8*time.Hour + 55*time.Minute)
	req := connect.NewRequest(&trainsv1.SearchJourneysRequest{
		OriginStopId:      "SA",
		DestinationStopId: "SC",
		Time:              when.Format(time.RFC3339),
	})

	resp, err := client.SearchJourneys(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetJourneys())
	j := resp.Msg.GetJourneys()[0]
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
