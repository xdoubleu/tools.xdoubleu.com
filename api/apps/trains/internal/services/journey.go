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

// ErrRouterWarmingUp is returned by SearchJourneys before the in-memory CSA
// index has been built for the first time. Building it is a multi-query,
// whole-window scan (see RefreshWindow) — far too heavy to run inside a
// request handler, where it would blow past the edge proxy's response
// timeout and have the connection reset with no trace
// (docs/adr-0017-long-request-handler-deadlines.md). The router is warmed at
// startup and on a schedule instead (issue #1484).
var ErrRouterWarmingUp = errors.New("journey router is warming up")

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
	// refreshGroup collapses concurrent rebuilds (the startup warm-up racing
	// the first scheduled refresh, say) into a single window scan.
	refreshGroup singleflight.Group
}

func NewJourneyService(
	logger *slog.Logger, repos *repositories.Repositories,
) *JourneyService {
	//nolint:exhaustruct //index is built lazily on first search or refresh
	return &JourneyService{logger: logger, repos: repos}
}

// SearchJourneys returns a Pareto set of journeys from originStopID to
// destStopID for the given time. It reads the in-memory index built by the
// background warm-up/refresh; if that hasn't happened yet it returns
// ErrRouterWarmingUp rather than building it in-request.
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

// RefreshOnly rebuilds the router index, discarding the built index —
// matches the func(context.Context) error shape jobs.RouterRefreshJob
// expects.
func (s *JourneyService) RefreshOnly(ctx context.Context) error {
	_, err := s.Refresh(ctx)
	return err
}

// Refresh rebuilds the router index from the current rolling window and
// swaps it in atomically. Concurrent callers share one rebuild.
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
