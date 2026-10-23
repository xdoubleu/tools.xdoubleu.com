package services

import (
	"context"
	"errors"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// onTimeThresholdSeconds: delays below this render as on time.
const onTimeThresholdSeconds = 60

// replanner is the slice of JourneyService the alternative evaluator needs.
type replanner interface {
	SearchJourneys(
		ctx context.Context,
		originStopID, destStopID string,
		when time.Time,
		arriveBy bool,
	) ([]models.JourneyOption, error)
	MinTransferSeconds(fromStopID, toStopID string) int
}

// JourneyDetailService rebuilds a searched journey's live state from its
// journey_id; nothing is persisted.
type JourneyDetailService struct {
	client   irail.Client
	stations *StationsService
	// replan re-queries iRail when the journey breaks; nil disables it.
	replan replanner
}

func NewJourneyDetailService(
	client irail.Client, stations *StationsService, replan replanner,
) *JourneyDetailService {
	return &JourneyDetailService{client: client, stations: stations, replan: replan}
}

// GetJourneyDetail rebuilds the live state of each leg in journeyID.
func (s *JourneyDetailService) GetJourneyDetail(
	ctx context.Context, journeyID string,
) (*models.JourneyDetail, error) {
	refs, err := DecodeJourneyID(journeyID)
	if err != nil {
		return nil, err
	}

	legs := make([]models.LegDetail, len(refs))
	for i, ref := range refs {
		leg, buildErr := s.buildLeg(ctx, ref)
		if buildErr != nil {
			return nil, buildErr
		}
		legs[i] = leg
	}

	return &models.JourneyDetail{
		Legs:          legs,
		DepartureTime: refs[0].BoardTime,
		ArrivalTime:   refs[len(refs)-1].AlightTime,
		Alternative:   s.evaluateAlternative(ctx, refs, legs),
	}, nil
}

func (s *JourneyDetailService) buildLeg(
	ctx context.Context, ref LegRef,
) (models.LegDetail, error) {
	vehicle, err := s.client.Vehicle(ctx, ref.VehicleID, ref.BoardTime)
	if errors.Is(err, irail.ErrNotFound) {
		// The train is gone from iRail; render the leg without stop detail
		// rather than failing the whole journey.
		return models.LegDetail{
			TripShortName:  vehicleNumber(ref.VehicleID),
			RouteShortName: vehicleType(ref.VehicleID),
			Cancelled:      false,
			Stops:          nil,
			Alerts:         nil,
			Headsign:       "",
		}, nil
	}
	if err != nil {
		return models.LegDetail{}, err
	}

	boardIdx, alightIdx := legRange(vehicle.Calls, ref.BoardStopID, ref.AlightStopID)
	cancelled := vehicle.Canceled()
	leg := models.LegDetail{
		TripShortName:  vehicle.Num,
		RouteShortName: vehicle.Type,
		Headsign:       lastCallName(vehicle.Calls),
		Cancelled:      cancelled,
		Stops:          nil,
		Alerts:         nil,
	}
	if boardIdx < 0 || alightIdx < 0 || alightIdx < boardIdx {
		return leg, nil
	}

	for i := boardIdx; i <= alightIdx; i++ {
		leg.Stops = append(leg.Stops, s.stopDetail(ctx, vehicle.Calls[i], ref, cancelled))
	}
	return leg, nil
}

func (s *JourneyDetailService) stopDetail(
	ctx context.Context, call irail.Call, ref LegRef, cancelled bool,
) models.StopDetail {
	name := call.Name
	if st, ok, err := s.stations.ByID(ctx, call.StationID); err == nil && ok {
		name = st.DisplayName
	}
	d := models.StopDetail{
		StopID:             call.StationID,
		StopName:           name,
		Platform:           call.Platform,
		ScheduledArrival:   nil,
		ScheduledDeparture: nil,
		State:              stopState(call, cancelled),
		ArrivalDelay:       nil,
		DepartureDelay:     nil,
		IsBoardStop:        call.StationID == ref.BoardStopID,
		IsAlightStop:       call.StationID == ref.AlightStopID,
	}
	if !call.ScheduledArrival.IsZero() {
		arr := call.ScheduledArrival
		d.ScheduledArrival = &arr
		delay := call.ArrivalDelay
		d.ArrivalDelay = &delay
	}
	if !call.ScheduledDeparture.IsZero() {
		dep := call.ScheduledDeparture
		d.ScheduledDeparture = &dep
		delay := call.DepartureDelay
		d.DepartureDelay = &delay
	}
	return d
}

// stopState maps iRail's per-stop flags: a cancelled stop on a running train
// is skipped; delays below the threshold count as on time.
func stopState(call irail.Call, cancelled bool) models.DelayState {
	switch {
	case cancelled:
		return models.DelayCancelled
	case call.ArrivalCanceled || call.DepartureCanceled:
		return models.DelaySkipped
	case absInt(call.ArrivalDelay) >= onTimeThresholdSeconds ||
		absInt(call.DepartureDelay) >= onTimeThresholdSeconds:
		return models.DelayDelayed
	default:
		return models.DelayOnTime
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// legRange finds [boardIdx, alightIdx] of the boarded portion of a run.
func legRange(calls []irail.Call, boardStopID, alightStopID string) (int, int) {
	boardIdx, alightIdx := -1, -1
	for i, c := range calls {
		if c.StationID == boardStopID && boardIdx == -1 {
			boardIdx = i
		}
		if c.StationID == alightStopID {
			alightIdx = i
		}
	}
	return boardIdx, alightIdx
}

func lastCallName(calls []irail.Call) string {
	if len(calls) == 0 {
		return ""
	}
	return calls[len(calls)-1].Name
}

// vehicleType and vehicleNumber split "BE.NMBS.IC530" into "IC" and "530".
func vehicleType(vehicleID string) string {
	short := irail.ShortName(vehicleID)
	for i, r := range short {
		if r >= '0' && r <= '9' {
			return short[:i]
		}
	}
	return short
}

func vehicleNumber(vehicleID string) string {
	short := irail.ShortName(vehicleID)
	return short[len(vehicleType(vehicleID)):]
}
