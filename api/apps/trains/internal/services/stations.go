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
	StopID string
	NameNL string
	NameFR string
	NameEN string
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

	q := strings.ToLower(strings.TrimSpace(query))
	matches := make([]Station, 0, len(stops))
	for _, stop := range stops {
		if stop.LocationType != stationLocationType {
			continue
		}
		if !nameMatches(stop, q) {
			continue
		}
		matches = append(matches, Station{
			StopID: stop.StopID,
			NameNL: stop.NameNL,
			NameFR: stop.NameFR,
			NameEN: stop.NameEN,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].NameFR < matches[j].NameFR
	})
	if len(matches) > maxStationResults {
		matches = matches[:maxStationResults]
	}
	return matches, nil
}

// nameMatches reports whether q (already lowercased) is a substring of
// stop's name in any of the three languages. An empty q always matches.
func nameMatches(stop models.Stop, q string) bool {
	if q == "" {
		return true
	}
	for _, name := range []string{stop.NameNL, stop.NameFR, stop.NameEN} {
		if strings.Contains(strings.ToLower(name), q) {
			return true
		}
	}
	return false
}
