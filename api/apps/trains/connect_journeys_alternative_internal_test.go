package trains

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// TestProtoJourneyAlternative maps the domain re-plan onto the wire message
// the live journey page renders (issue #1395): nil in, nil out; a populated
// alternative carries its reason, origin name and nested journey with
// RFC3339 leg times.
func TestProtoJourneyAlternative(t *testing.T) {
	assert.Nil(t, protoJourneyAlternative(nil))

	dep := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	arr := dep.Add(40 * time.Minute)
	out := protoJourneyAlternative(&models.JourneyAlternative{
		Reason:       "IC900 is cancelled",
		FromStopName: "Mechelen",
		Journey: &models.JourneyOption{
			Legs: []models.JourneyOptionLeg{{
				TripShortName:  "IC1717",
				RouteShortName: "IC",
				Headsign:       "Ghent",
				BoardStopID:    "M1",
				BoardStopName:  "Mechelen",
				BoardPlatform:  "3",
				BoardTime:      dep,
				AlightStopID:   "G1",
				AlightStopName: "Ghent",
				AlightPlatform: "7",
				AlightTime:     arr,
			}},
			DepartureTime: dep,
			ArrivalTime:   arr,
			Transfers:     1,
			JourneyID:     "replan-id",
		},
	})

	require.NotNil(t, out)
	assert.Equal(t, "IC900 is cancelled", out.GetReason())
	assert.Equal(t, "Mechelen", out.GetFromStopName())
	require.NotNil(t, out.GetJourney())
	assert.Equal(t, "replan-id", out.GetJourney().GetJourneyId())
	assert.Equal(t, int32(1), out.GetJourney().GetTransfers())
	require.Len(t, out.GetJourney().GetLegs(), 1)
	leg := out.GetJourney().GetLegs()[0]
	assert.Equal(t, "IC1717", leg.GetTripShortName())
	assert.Equal(t, "2026-09-07T08:00:00Z", leg.GetBoardTime())
	assert.Equal(t, "2026-09-07T08:40:00Z", leg.GetAlightTime())
}

// TestProtoJourneyAlternative_ReasonOnly covers the router-found-nothing
// path — the reason still crosses the wire without a journey.
func TestProtoJourneyAlternative_ReasonOnly(t *testing.T) {
	//nolint:exhaustruct //Journey intentionally nil, under test
	out := protoJourneyAlternative(&models.JourneyAlternative{
		Reason:       "Arrival at Ghent is running 20 min late",
		FromStopName: "Mechelen",
	})
	require.NotNil(t, out)
	assert.Nil(t, out.GetJourney())
	assert.Contains(t, out.GetReason(), "20 min late")
}
