package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// ErrUnknownStation is returned when an origin or destination isn't a station.
var ErrUnknownStation = errors.New("trains: unknown station id")

// minTransferSeconds is the change time assumed when judging whether a delayed
// arrival still makes its connection.
const minTransferSeconds = 180

// brusselsLoc is the timezone passengers read times in.
//
//nolint:gochecknoglobals //fixed timezone
var brusselsLoc = mustLoadLocation("Europe/Brussels")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// JourneyService answers SearchJourneys from iRail's connections.
type JourneyService struct {
	client   irail.Client
	stations *StationsService
}

func NewJourneyService(
	client irail.Client, stations *StationsService,
) *JourneyService {
	return &JourneyService{client: client, stations: stations}
}

// SearchJourneys returns the journeys iRail plans around the given time,
// ordered by departure.
func (s *JourneyService) SearchJourneys(
	ctx context.Context,
	originStopID, destStopID string,
	when time.Time,
	arriveBy bool,
) ([]models.JourneyOption, error) {
	for _, id := range []string{originStopID, destStopID} {
		if _, ok, err := s.stations.ByID(ctx, id); err != nil {
			return nil, err
		} else if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownStation, id)
		}
	}

	conns, err := s.client.Connections(ctx, originStopID, destStopID, when, arriveBy)
	if err != nil {
		return nil, err
	}

	out := make([]models.JourneyOption, 0, len(conns))
	for _, c := range conns {
		if len(c.Legs) == 0 {
			continue
		}
		out = append(out, s.option(ctx, c.Legs))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DepartureTime.Before(out[j].DepartureTime)
	})
	return out, nil
}

// MinTransferSeconds is the assumed change time between two stations.
func (s *JourneyService) MinTransferSeconds(_, _ string) int {
	return minTransferSeconds
}

func (s *JourneyService) option(
	ctx context.Context,
	legs []irail.Leg,
) models.JourneyOption {
	optLegs := make([]models.JourneyOptionLeg, len(legs))
	refs := make([]LegRef, len(legs))
	for i, l := range legs {
		optLegs[i] = models.JourneyOptionLeg{
			TripShortName:  l.Number,
			RouteShortName: l.Type,
			Headsign:       l.Direction,
			BoardStopID:    l.Board.StationID,
			BoardStopName:  s.stationName(ctx, l.Board),
			BoardPlatform:  l.Board.Platform,
			BoardTime:      l.Board.Time,
			AlightStopID:   l.Alight.StationID,
			AlightStopName: s.stationName(ctx, l.Alight),
			AlightPlatform: l.Alight.Platform,
			AlightTime:     l.Alight.Time,
		}
		refs[i] = LegRef{
			VehicleID:    l.VehicleID,
			BoardStopID:  l.Board.StationID,
			BoardTime:    l.Board.Time,
			AlightStopID: l.Alight.StationID,
			AlightTime:   l.Alight.Time,
		}
	}
	return models.JourneyOption{
		Legs:          optLegs,
		DepartureTime: legs[0].Board.Time,
		ArrivalTime:   legs[len(legs)-1].Alight.Time,
		Transfers:     len(legs) - 1,
		JourneyID:     EncodeJourneyID(refs),
	}
}

// stationName prefers the app's canonical display name over iRail's
// single-language one.
func (s *JourneyService) stationName(ctx context.Context, e irail.Endpoint) string {
	if st, ok, err := s.stations.ByID(ctx, e.StationID); err == nil && ok {
		return st.DisplayName
	}
	return e.Name
}
