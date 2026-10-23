package trains_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

const (
	brusselsNorth = "008812005"
	liege         = "008841004"
)

func TestSearchStations_Handler(t *testing.T) {
	client := newTrainsTestClient(t)

	resp, err := client.SearchStations(
		context.Background(), &trainsv1.SearchStationsRequest{Query: "bruxelles-nord"},
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetStations())
	st := resp.GetStations()[0]
	assert.Equal(t, brusselsNorth, st.GetStopId())
	assert.Equal(t, "Brussel-Noord", st.GetNameNl())
	assert.Equal(t, "Bruxelles-Nord", st.GetNameFr())
	assert.Equal(t, "Brussels-North", st.GetNameEn())
	assert.Equal(t, "Bruxelles-Nord / Brussel-Noord / Brussels-North", st.GetDisplayName())
}

func TestSearchJourneys_Handler_UnknownStation(t *testing.T) {
	client := newTrainsTestClient(t)
	_, err := client.SearchJourneys(context.Background(), &trainsv1.SearchJourneysRequest{
		OriginStopId: "does-not-exist", DestinationStopId: liege,
	})
	assert.Equal(t, connect.CodeNotFound, connectCode(t, err))
}

func TestSearchJourneys_Handler_InvalidTime(t *testing.T) {
	client := newTrainsTestClient(t)
	_, err := client.SearchJourneys(context.Background(), &trainsv1.SearchJourneysRequest{
		OriginStopId: brusselsNorth, DestinationStopId: liege, Time: "not-a-timestamp",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, err))
}

// TestJourneySearchToDetail follows a searched journey into its live detail,
// the path the /trains pages take.
func TestJourneySearchToDetail(t *testing.T) {
	ctx := context.Background()
	client := newTrainsTestClient(t)

	resp, err := client.SearchJourneys(ctx, &trainsv1.SearchJourneysRequest{
		OriginStopId:      brusselsNorth,
		DestinationStopId: liege,
		Time:              time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC).Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetJourneys(), 6)

	j := resp.GetJourneys()[0]
	require.NotEmpty(t, j.GetLegs())
	leg := j.GetLegs()[0]
	assert.Equal(t, brusselsNorth, leg.GetBoardStopId())
	assert.NotEmpty(t, leg.GetBoardTime())
	assert.NotEmpty(t, leg.GetAlightTime())
	assert.NotEmpty(t, j.GetJourneyId())
	assert.Equal(t, int32(len(j.GetLegs())-1), j.GetTransfers())

	// The recorded responses only cover train IC 530, so a detail request for
	// another train renders without stops, and an unknown train is no error.
	detail, err := client.GetJourneyDetail(ctx, &trainsv1.GetJourneyDetailRequest{
		JourneyId: j.GetJourneyId(),
	})
	require.NoError(t, err)
	assert.Equal(t, j.GetJourneyId(), detail.GetJourney().GetJourneyId())
	assert.Len(t, detail.GetJourney().GetLegs(), len(j.GetLegs()))
}

func TestGetJourneyDetail_Handler_InvalidIDs(t *testing.T) {
	client := newTrainsTestClient(t)
	for _, id := range []string{"", "!!not-valid!!"} {
		_, err := client.GetJourneyDetail(
			context.Background(), &trainsv1.GetJourneyDetailRequest{JourneyId: id},
		)
		assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, err), id)
	}
}
