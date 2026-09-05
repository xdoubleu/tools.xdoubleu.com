package services

import (
	"context"
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// stationLocationType is the GTFS location_type for a station (as opposed
// to a platform) — see models.Stop.
const stationLocationType = 1

// maxStationResults caps a single SearchStations response — enough for a
// type-ahead dropdown without shipping all 652 stops to the client.
const maxStationResults = 20

// Station is a location_type=1 stop a passenger can pick as an origin or
// destination.
type Station struct {
	StopID string
	Name   string
}

// StationsService answers SearchStations for the /trains station pickers.
type StationsService struct {
	repos *repositories.Repositories
}

func NewStationsService(repos *repositories.Repositories) *StationsService {
	return &StationsService{repos: repos}
}

// SearchStations returns up to maxStationResults stations whose name
// contains query, case-insensitively, ordered alphabetically. An empty
// query returns the first page of all stations in the same order.
func (s *StationsService) SearchStations(
	ctx context.Context, query string,
) ([]Station, error) {
	stops, err := s.repos.Feed.AllStops(ctx)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	matches := make([]Station, 0, len(stops))
	for _, stop := range stops {
		if stop.LocationType != stationLocationType {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(stop.Name), q) {
			continue
		}
		matches = append(matches, Station{StopID: stop.StopID, Name: stop.Name})
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	if len(matches) > maxStationResults {
		matches = matches[:maxStationResults]
	}
	return matches, nil
}
