package services

import (
	"context"
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/trains/internal/models"
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
// of the three languages contains query, case-insensitively, ordered
// alphabetically by the French name. An empty query returns the first page
// of all stations in the same order.
func (s *StationsService) SearchStations(
	ctx context.Context, query string,
) ([]Station, error) {
	stops, err := s.repos.Feed.AllStops(ctx)
	if err != nil {
		return nil, err
	}
	return matchStations(stops, query), nil
}

// matchStations filters stops down to stations matching query (empty query
// matches everything), dedupes stations that share a non-empty UIC (distinct
// stop_ids for the same physical station, e.g. a re-numbered or dual-feed
// entry — keeping the lexicographically lowest stop_id for determinism), and
// sorts/caps the result. A stop whose UIC couldn't be parsed (empty) is never
// merged with another empty-UIC stop — collapsing all of those together
// would falsely correlate unrelated stations.
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
