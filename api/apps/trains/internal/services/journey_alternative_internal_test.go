package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

// base is a fixed wall-clock the fixtures below hang scheduled times off.
//
//nolint:gochecknoglobals //fixed test anchor
var altBase = time.Date(2026, 6, 1, 8, 0, 0, 0, brusselsLoc)

func tPtr(t time.Time) *time.Time { return &t }

func iPtr(n int) *int { return &n }

// twoLegRefs is a Brussels→transfer→destination itinerary: leg A arrives at
// stop M at 08:30, leg B leaves M at 08:40.
func twoLegRefs() []LegRef {
	return []LegRef{
		{
			TripShortName: "A", BoardStopID: "P1",
			BoardTime:    altBase,
			AlightStopID: "M", AlightTime: altBase.Add(30 * time.Minute),
		},
		{
			TripShortName: "B", BoardStopID: "M",
			BoardTime:    altBase.Add(40 * time.Minute),
			AlightStopID: "P9", AlightTime: altBase.Add(90 * time.Minute),
		},
	}
}

func legDetail(
	trip string,
	cancelled bool,
	stops ...models.StopDetail,
) models.LegDetail {
	return models.LegDetail{
		TripShortName: trip, RouteShortName: "IC", Headsign: "H",
		Cancelled: cancelled, Stops: stops, Alerts: nil,
	}
}

func stop(
	id, name string,
	state models.DelayState,
	arr, dep *time.Time,
	arrDelay *int,
) models.StopDetail {
	//nolint:exhaustruct //only the fields the detector reads are set
	return models.StopDetail{
		StopID: id, StopName: name, State: state,
		ScheduledArrival: arr, ScheduledDeparture: dep, ArrivalDelay: arrDelay,
	}
}

// fixedTransfer stands in for JourneyService.MinTransferSeconds with a
// constant 5-minute change everywhere.
func fixedTransfer(_, _ string) int { return 300 }

func TestDetectJourneyBreak_NoLiveDataNeverFires(t *testing.T) {
	refs := twoLegRefs()
	aArr := altBase.Add(30 * time.Minute)
	bDep := altBase.Add(40 * time.Minute)
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop("M", "Mechelen", models.DelayUnknown, tPtr(aArr), nil, nil),
		),
		legDetail(
			"B",
			false,
			stop("M", "Mechelen", models.DelayUnknown, nil, tPtr(bDep), nil),
		),
	}

	_, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	assert.False(t, ok, "NO_DATA on every call must not trigger an alternative")
}

func TestDetectJourneyBreak_MissedConnection(t *testing.T) {
	refs := twoLegRefs()
	aArr := altBase.Add(30 * time.Minute)
	bDep := altBase.Add(40 * time.Minute)
	// Leg A now arrives 8 minutes late (08:38); with a 5-minute change it
	// can't make leg B's 08:40 departure.
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop("M", "Mechelen", models.DelayDelayed, tPtr(aArr), nil, iPtr(480)),
		),
		legDetail(
			"B",
			false,
			stop("M", "Mechelen", models.DelayUnknown, nil, tPtr(bDep), nil),
		),
	}

	brk, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	require.True(t, ok)
	assert.Equal(t, "M", brk.fromStopID)
	assert.Equal(t, "Mechelen", brk.fromStopName)
	assert.Equal(t, "P9", brk.destStopID)
	assert.Equal(t, aArr.Add(8*time.Minute), brk.at)
	assert.Contains(t, brk.reason, "08:40")
	assert.Contains(t, brk.reason, "Mechelen")
}

func TestDetectJourneyBreak_DelayedButConnectionStillMade(t *testing.T) {
	refs := twoLegRefs()
	aArr := altBase.Add(30 * time.Minute)
	bDep := altBase.Add(40 * time.Minute)
	// 2 minutes late, 5-minute change, 10-minute window — still fine.
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop("M", "Mechelen", models.DelayDelayed, tPtr(aArr), nil, iPtr(120)),
		),
		legDetail(
			"B",
			false,
			stop("M", "Mechelen", models.DelayOnTime, nil, tPtr(bDep), nil),
		),
	}

	_, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	assert.False(t, ok)
}

func TestDetectJourneyBreak_CancelledLeg(t *testing.T) {
	refs := twoLegRefs()
	bDep := altBase.Add(40 * time.Minute)
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				tPtr(altBase.Add(30*time.Minute)),
				nil,
				nil,
			),
		),
		legDetail(
			"B",
			true,
			stop("M", "Mechelen", models.DelayCancelled, nil, tPtr(bDep), nil),
		),
	}

	brk, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	require.True(t, ok)
	assert.Equal(t, "M", brk.fromStopID)
	assert.True(t, brk.deadTrips["B"])
	assert.Contains(t, brk.reason, "cancelled")
	// Re-plan starts at leg A's live arrival at M.
	assert.Equal(t, altBase.Add(30*time.Minute), brk.at)
}

func TestDetectJourneyBreak_BoardStopSkipped(t *testing.T) {
	refs := twoLegRefs()
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				tPtr(altBase.Add(30*time.Minute)),
				nil,
				nil,
			),
		),
		legDetail(
			"B",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelaySkipped,
				nil,
				tPtr(altBase.Add(40*time.Minute)),
				nil,
			),
		),
	}

	brk, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	require.True(t, ok)
	assert.Contains(t, brk.reason, "no longer stops at Mechelen")
	assert.True(t, brk.deadTrips["B"])
}

func TestDetectJourneyBreak_FinalArrivalSlip(t *testing.T) {
	refs := twoLegRefs()
	pArr := altBase.Add(90 * time.Minute)
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				tPtr(altBase.Add(30*time.Minute)),
				nil,
				nil,
			),
		),
		legDetail(
			"B",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				nil,
				tPtr(altBase.Add(40*time.Minute)),
				nil,
			),
			stop("P9", "Ghent", models.DelayDelayed, tPtr(pArr), nil, iPtr(1200)),
		),
	}

	brk, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	require.True(t, ok)
	assert.Contains(t, brk.reason, "Ghent")
	assert.Equal(t, "M", brk.fromStopID) // board stop of the last leg
}

func TestDetectJourneyBreak_FinalSlipUnderThresholdDoesNotFire(t *testing.T) {
	refs := twoLegRefs()
	pArr := altBase.Add(90 * time.Minute)
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				tPtr(altBase.Add(30*time.Minute)),
				nil,
				nil,
			),
		),
		legDetail(
			"B",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				nil,
				tPtr(altBase.Add(40*time.Minute)),
				nil,
			),
			stop("P9", "Ghent", models.DelayDelayed, tPtr(pArr), nil, iPtr(300)),
		),
	}

	_, ok := detectJourneyBreak(refs, legs, fixedTransfer)
	assert.False(t, ok)
}

func TestHumanizeDuration(t *testing.T) {
	assert.Equal(t, "1 min", humanizeDuration(10*time.Second))
	assert.Equal(t, "4 min", humanizeDuration(3*time.Minute+40*time.Second))
	assert.Equal(t, "20 min", humanizeDuration(20*time.Minute))
}

// stubReplanner is a canned replanner for evaluateAlternative's paths.
type stubReplanner struct {
	journeys []csa.Journey
	err      error
}

func (s stubReplanner) SearchJourneys(
	_ context.Context, _, _ string, _ time.Time, _ bool,
) ([]csa.Journey, error) {
	return s.journeys, s.err
}

func (s stubReplanner) MinTransferSeconds(_, _ string) int { return 300 }

func brokenTwoLeg() ([]LegRef, []models.LegDetail) {
	refs := twoLegRefs()
	aArr := altBase.Add(30 * time.Minute)
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop("M", "Mechelen", models.DelayDelayed, tPtr(aArr), nil, iPtr(480)),
		),
		legDetail(
			"B",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayUnknown,
				nil,
				tPtr(altBase.Add(40*time.Minute)),
				nil,
			),
		),
	}
	return refs, legs
}

func TestEvaluateAlternative_NilReplanIsNoop(t *testing.T) {
	s := &JourneyDetailService{replan: nil} //nolint:exhaustruct //only replan matters
	refs, legs := brokenTwoLeg()
	assert.Nil(t, s.evaluateAlternative(context.Background(), refs, legs))
}

func TestEvaluateAlternative_ReasonSurvivesRouterFailure(t *testing.T) {
	//nolint:exhaustruct //only replan matters
	s := &JourneyDetailService{replan: stubReplanner{err: errors.New("warming up")}}
	refs, legs := brokenTwoLeg()

	alt := s.evaluateAlternative(context.Background(), refs, legs)
	require.NotNil(t, alt)
	assert.NotEmpty(t, alt.Reason)
	assert.Nil(t, alt.Journey, "no journey when the router can't answer")
}

func TestEvaluateAlternative_SkipsCandidateReusingDeadTrip(t *testing.T) {
	refs := twoLegRefs()
	legs := []models.LegDetail{
		legDetail(
			"A",
			false,
			stop(
				"M",
				"Mechelen",
				models.DelayOnTime,
				tPtr(altBase.Add(30*time.Minute)),
				nil,
				nil,
			),
		),
		legDetail(
			"B",
			true,
			stop(
				"M",
				"Mechelen",
				models.DelayCancelled,
				nil,
				tPtr(altBase.Add(40*time.Minute)),
				nil,
			),
		),
	}
	//nolint:exhaustruct //only the fields reusesDeadTrip reads are set
	dead := csa.Journey{Legs: []csa.Leg{{TripShortName: "B"}}}
	//nolint:exhaustruct //only the fields toJourneyOption copies are set
	good := csa.Journey{
		Legs: []csa.Leg{{
			TripShortName: "C", BoardStopID: "M", AlightStopID: "P9",
			BoardTime: altBase.Add(time.Hour), AlightTime: altBase.Add(2 * time.Hour),
		}},
		DepartureTime: altBase.Add(time.Hour),
		ArrivalTime:   altBase.Add(2 * time.Hour),
	}
	//nolint:exhaustruct //only replan matters
	s := &JourneyDetailService{
		replan: stubReplanner{journeys: []csa.Journey{dead, good}},
	}

	alt := s.evaluateAlternative(context.Background(), refs, legs)
	require.NotNil(t, alt)
	require.NotNil(t, alt.Journey)
	require.Len(t, alt.Journey.Legs, 1)
	assert.Equal(t, "C", alt.Journey.Legs[0].TripShortName)
	assert.NotEmpty(t, alt.Journey.JourneyID)
}
