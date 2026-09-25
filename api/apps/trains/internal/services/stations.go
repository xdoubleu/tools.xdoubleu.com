package services

import (
	"context"
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// stationLocationType is GTFS location_type for a station.
const stationLocationType = 1

// maxStationResults caps a SearchStations response for a type-ahead.
const maxStationResults = 20

// Station is a location_type=1 stop a passenger can pick.
type Station struct {
	StopID      string
	NameNL      string
	NameFR      string
	NameEN      string
	DisplayName string
}

// StationsService answers SearchStations for the /trains station pickers.
type StationsService struct {
	repos *repositories.Repositories
}

func NewStationsService(repos *repositories.Repositories) *StationsService {
	return &StationsService{repos: repos}
}

// SearchStations returns up to maxStationResults stations whose name in any
// language contains query (case-insensitive), sorted by French name.
func (s *StationsService) SearchStations(
	ctx context.Context, query string,
) ([]Station, error) {
	stops, err := s.repos.Feed.AllStops(ctx)
	if err != nil {
		return nil, err
	}
	return matchStations(stops, query), nil
}

// matchStations filters stations by query, dedupes those sharing a non-empty
// UIC (keeping the lowest stop_id), then sorts and caps. Empty-UIC stops are
// never merged with each other.
func matchStations(stops []models.Stop, query string) []Station {
	q := strings.ToLower(strings.TrimSpace(query))
	byUIC := make(map[string]Station)
	var noUIC []Station
	for _, stop := range stops {
		if stop.LocationType != stationLocationType {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(stop.NameNL), q) &&
			!strings.Contains(strings.ToLower(stop.NameFR), q) &&
			!strings.Contains(strings.ToLower(stop.NameEN), q) {
			continue
		}
		station := Station{
			StopID:      stop.StopID,
			NameNL:      stop.NameNL,
			NameFR:      stop.NameFR,
			NameEN:      stop.NameEN,
			DisplayName: stop.DisplayName,
		}
		if stop.UIC == "" {
			noUIC = append(noUIC, station)
			continue
		}
		if existing, ok := byUIC[stop.UIC]; !ok || station.StopID < existing.StopID {
			byUIC[stop.UIC] = station
		}
	}

	matches := make([]Station, 0, len(byUIC)+len(noUIC))
	for _, station := range byUIC {
		matches = append(matches, station)
	}
	matches = append(matches, noUIC...)

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].NameFR < matches[j].NameFR
	})
	if len(matches) > maxStationResults {
		matches = matches[:maxStationResults]
	}
	return matches
}
