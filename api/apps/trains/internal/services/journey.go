package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

// routerWindowDays bounds how many service days' worth of stop_times the
// router holds in memory at once. The full feed is ~2.2M stop_times across
// a year; a rolling window is what keeps this comfortably inside the
// deployed container's GOMEMLIMIT (300MiB, config/deploy.api.yml) instead
// of growing without bound (issue #1391).
const routerWindowDays = 14

// brusselsLoc is the feed's local timezone — GTFS stop_times are local
// wall-clock offsets from a service date's midnight.
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

// JourneyService answers SearchJourneys over an in-memory CSA index built
// from a rolling window of the ingested timetable. Routing is timetable-only
// — no realtime overlay (that's #1388 slice 5/6, applied on top of the same
// connection array rather than baked in here).
type JourneyService struct {
	logger *slog.Logger
	repos  *repositories.Repositories

	mu    sync.RWMutex
	index *csa.Index
}

func NewJourneyService(
	logger *slog.Logger, repos *repositories.Repositories,
) *JourneyService {
	//nolint:exhaustruct //index is built lazily on first search or refresh
	return &JourneyService{logger: logger, repos: repos}
}

// SearchJourneys returns a Pareto set of journeys from originStopID to
// destStopID for the given time, building or refreshing the in-memory
// index first if it's stale or missing.
func (s *JourneyService) SearchJourneys(
	ctx context.Context,
	originStopID, destStopID string,
	when time.Time,
	arriveBy bool,
) ([]csa.Journey, error) {
	idx, err := s.currentIndex(ctx)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return nil, nil
	}
	return idx.SearchJourneys(originStopID, destStopID, when, arriveBy)
}

func (s *JourneyService) currentIndex(ctx context.Context) (*csa.Index, error) {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx != nil {
		return idx, nil
	}
	return s.Refresh(ctx)
}

// RefreshOnly rebuilds the router index, discarding the built index —
// matches the func(context.Context) error shape jobs.RouterRefreshJob
// expects.
func (s *JourneyService) RefreshOnly(ctx context.Context) error {
	_, err := s.Refresh(ctx)
	return err
}

// Refresh rebuilds the router index from the current rolling window and
// swaps it in atomically.
func (s *JourneyService) Refresh(ctx context.Context) (*csa.Index, error) {
	const oneDay = 24 * time.Hour
	windowStart := time.Now().In(brusselsLoc).Truncate(oneDay)
	return s.RefreshWindow(ctx, windowStart)
}

// RefreshWindow rebuilds the router index for a caller-chosen window start
// — Refresh's production path always anchors this to "today", but tests
// need a fixed window aligned to fixture dates.
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
		brusselsLoc,
		windowStart,
		toCSAStops(stops),
		toCSATransfers(transfers),
		toCSAInstances(active),
		toCSAPatterns(stopTimes),
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
