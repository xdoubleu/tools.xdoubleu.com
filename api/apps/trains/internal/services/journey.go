package services

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

// ErrRouterWarmingUp is returned before the CSA index is first built. The
// build is too heavy for a request handler (it would outlive the proxy
// timeout, see docs/adr-0017), so it's warmed at startup and on a schedule.
var ErrRouterWarmingUp = errors.New("journey router is warming up")

// routerWindowDays bounds the in-memory stop_times window to keep the router
// inside the container's GOMEMLIMIT (config/deploy.api.yml).
const routerWindowDays = 14

// brusselsLoc is the feed's timezone; stop_times are offsets from local
// midnight.
//
//nolint:gochecknoglobals //fixed feed timezone
var brusselsLoc = mustLoadLocation("Europe/Brussels")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// JourneyService answers SearchJourneys over an in-memory CSA index of a
// rolling timetable window. Routing is timetable-only.
type JourneyService struct {
	logger *slog.Logger
	repos  *repositories.Repositories

	mu    sync.RWMutex
	index *csa.Index
	// refreshGroup collapses concurrent rebuilds into one window scan.
	refreshGroup singleflight.Group
}

func NewJourneyService(
	logger *slog.Logger, repos *repositories.Repositories,
) *JourneyService {
	//nolint:exhaustruct //index is built lazily on first search or refresh
	return &JourneyService{logger: logger, repos: repos}
}

// SearchJourneys returns a few journeys before and after the given time. It
// returns ErrRouterWarmingUp rather than building the index in-request.
func (s *JourneyService) SearchJourneys(
	_ context.Context,
	originStopID, destStopID string,
	when time.Time,
	arriveBy bool,
) ([]csa.Journey, error) {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx == nil {
		return nil, ErrRouterWarmingUp
	}
	return idx.SearchJourneys(originStopID, destStopID, when, arriveBy)
}

// MinTransferSeconds reports the minimum change time between two stops,
// falling back to the router default before the index is built.
func (s *JourneyService) MinTransferSeconds(fromStopID, toStopID string) int {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx == nil {
		return csa.DefaultMinTransferSeconds
	}
	return idx.MinTransferSeconds(fromStopID, toStopID)
}

// RefreshOnly rebuilds the index, matching jobs.RouterRefreshJob's func shape.
func (s *JourneyService) RefreshOnly(ctx context.Context) error {
	_, err := s.Refresh(ctx)
	return err
}

// Refresh rebuilds the index for the current window and swaps it in
// atomically; concurrent callers share one rebuild.
func (s *JourneyService) Refresh(ctx context.Context) (*csa.Index, error) {
	_, err, _ := s.refreshGroup.Do("refresh", func() (any, error) {
		const oneDay = 24 * time.Hour
		windowStart := time.Now().In(brusselsLoc).Truncate(oneDay)
		return s.RefreshWindow(ctx, windowStart)
	})
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	return idx, nil
}

// RefreshWindow rebuilds the index for a caller-chosen window start (tests
// align it to fixture dates).
func (s *JourneyService) RefreshWindow(
	ctx context.Context, windowStart time.Time,
) (*csa.Index, error) {
	windowEnd := windowStart.AddDate(0, 0, routerWindowDays-1)

	stops, err := s.repos.Feed.AllStops(ctx)
	if err != nil {
		return nil, err
	}
	transfers, err := s.repos.Feed.AllTransfers(ctx)
	if err != nil {
		return nil, err
	}
	active, err := s.repos.Feed.ActiveTripsInWindow(ctx, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	tripIDSet := make(map[string]bool, len(active))
	for _, a := range active {
		tripIDSet[a.TripID] = true
	}
	tripIDs := make([]string, 0, len(tripIDSet))
	for id := range tripIDSet {
		tripIDs = append(tripIDs, id)
	}
	stopTimes, err := s.repos.Feed.StopTimesForTrips(ctx, tripIDs)
	if err != nil {
		return nil, err
	}

	idx := csa.Build(
		brusselsLoc, windowStart, stops, transfers, active, stopTimes,
	)

	s.mu.Lock()
	s.index = idx
	s.mu.Unlock()

	s.logger.InfoContext(ctx, "trains: router index rebuilt",
		slog.Int("stops", len(stops)),
		slog.Int("active_trip_days", len(active)),
		slog.Time("window_start", windowStart),
		slog.Int("window_days", routerWindowDays),
	)
	return idx, nil
}
