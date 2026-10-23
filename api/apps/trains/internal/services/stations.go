package services

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// maxStationResults caps a SearchStations response for a type-ahead.
const maxStationResults = 20

// stationsTTL is how long the station list is reused; stations rarely change.
const stationsTTL = 24 * time.Hour

// stationLanguages are the languages iRail names stations in.
//
//nolint:gochecknoglobals //fixed language set
var stationLanguages = []string{"nl", "fr", "en"}

// StationsService serves iRail's station list from memory, refreshed daily.
type StationsService struct {
	client irail.Client
	group  singleflight.Group

	mu       sync.RWMutex
	loadedAt time.Time
	list     []models.Station
	byID     map[string]models.Station
}

func NewStationsService(client irail.Client) *StationsService {
	//nolint:exhaustruct //the list is loaded lazily on first use
	return &StationsService{client: client}
}

// SearchStations returns up to maxStationResults stations whose name in any
// language contains query (case-insensitive), sorted by Dutch name.
func (s *StationsService) SearchStations(
	ctx context.Context, query string,
) ([]models.Station, error) {
	list, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return matchStations(list, query), nil
}

// ByID returns the station with the given UIC id.
func (s *StationsService) ByID(
	ctx context.Context, id string,
) (models.Station, bool, error) {
	_, byID, err := s.load(ctx)
	if err != nil {
		return models.Station{}, false, err
	}
	st, ok := byID[id]
	return st, ok, nil
}

func (s *StationsService) load(
	ctx context.Context,
) ([]models.Station, map[string]models.Station, error) {
	s.mu.RLock()
	list, byID, fresh := s.list, s.byID, time.Since(s.loadedAt) < stationsTTL
	s.mu.RUnlock()
	if list != nil && fresh {
		return list, byID, nil
	}

	_, err, _ := s.group.Do("load", func() (any, error) {
		return nil, s.refresh(ctx)
	})
	if err != nil {
		if list != nil {
			// Serve the stale list rather than failing every search.
			return list, byID, nil
		}
		return nil, nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.list, s.byID, nil
}

func (s *StationsService) refresh(ctx context.Context) error {
	names := make(map[string]map[string]string)
	for _, lang := range stationLanguages {
		stations, err := s.client.Stations(ctx, lang)
		if err != nil {
			return err
		}
		for _, st := range stations {
			if names[st.ID] == nil {
				names[st.ID] = make(map[string]string, len(stationLanguages))
			}
			names[st.ID][lang] = st.Name
		}
	}

	list := make([]models.Station, 0, len(names))
	byID := make(map[string]models.Station, len(names))
	for id, n := range names {
		st := models.Station{
			StopID:      id,
			NameNL:      n["nl"],
			NameFR:      n["fr"],
			NameEN:      n["en"],
			DisplayName: displayName(n["fr"], n["nl"], n["en"]),
		}
		list = append(list, st)
		byID[id] = st
	}
	sort.Slice(list, func(i, j int) bool {
		return lessStation(list[i], list[j])
	})

	s.mu.Lock()
	s.list, s.byID, s.loadedAt = list, byID, time.Now()
	s.mu.Unlock()
	return nil
}

// displayName joins the distinct non-empty names with " / ".
func displayName(names ...string) string {
	seen := make(map[string]bool, len(names))
	var out []string
	for _, n := range names {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return strings.Join(out, " / ")
}

func lessStation(a, b models.Station) bool {
	na, nb := strings.ToLower(a.NameNL), strings.ToLower(b.NameNL)
	if na != nb {
		return na < nb
	}
	return a.StopID < b.StopID
}

// matchStations filters the sorted list by query, then caps it.
func matchStations(list []models.Station, query string) []models.Station {
	q := strings.ToLower(strings.TrimSpace(query))
	matches := make([]models.Station, 0, maxStationResults)
	for _, st := range list {
		if q != "" &&
			!strings.Contains(strings.ToLower(st.NameNL), q) &&
			!strings.Contains(strings.ToLower(st.NameFR), q) &&
			!strings.Contains(strings.ToLower(st.NameEN), q) {
			continue
		}
		matches = append(matches, st)
		if len(matches) == maxStationResults {
			break
		}
	}
	return matches
}
