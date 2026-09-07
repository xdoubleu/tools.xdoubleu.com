package services

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// alertPollEvery is how many trip-update poll cycles elapse between alert
// polls. Alerts change far less often than delays (issue #1393), so with a
// 30s job cadence this polls alerts roughly every 2 minutes.
const alertPollEvery = 4

// RealtimeService polls the BMC gateway's GTFS-Realtime feeds and keeps the
// latest decoded state in memory. The snapshot is wholly replaced on every
// poll and nothing here is persisted — a later slice decides what, if
// anything, is worth writing down (issue #1393).
type RealtimeService struct {
	logger *slog.Logger
	bmc    bmc.Client

	mu        sync.RWMutex
	snapshot  models.Snapshot
	pollCount int
	onUpdate  []func()
}

func NewRealtimeService(logger *slog.Logger, bmcClient bmc.Client) *RealtimeService {
	//nolint:exhaustruct //snapshot/pollCount/onUpdate start zero-valued
	return &RealtimeService{logger: logger, bmc: bmcClient}
}

// OnUpdate registers fn to run after every successful Poll — used by
// JourneyWSService to push fresh journey detail to open sockets without
// waiting to be asked (issue #1394).
func (s *RealtimeService) OnUpdate(fn func()) {
	s.mu.Lock()
	s.onUpdate = append(s.onUpdate, fn)
	s.mu.Unlock()
}

// Snapshot returns the current realtime state. Safe for concurrent use.
func (s *RealtimeService) Snapshot() models.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Poll fetches trip-update on every call and alerts on a slower cadence,
// then replaces the in-memory snapshot wholesale. A rate-limited or 5xx
// gateway response is logged and this poll is skipped rather than failing
// the job — the next scheduled run retries, which is the backoff issue
// #1393 asks for given a job cadence far below the gateway's quota. Any
// other error (a decode failure, most likely) is returned so it surfaces
// on the monitoring page instead of failing silently.
func (s *RealtimeService) Poll(ctx context.Context) error {
	if s.bmc == nil {
		return nil
	}

	trips, err := s.fetchTripUpdates(ctx)
	if err != nil {
		if isBackoffable(err) {
			s.logger.Warn("trains: realtime trip-update poll backing off", "error", err)
			return nil
		}
		return err
	}

	s.mu.Lock()
	s.pollCount++
	fetchAlerts := s.pollCount%alertPollEvery == 1
	alerts := s.snapshot.Alerts
	s.mu.Unlock()

	if fetchAlerts {
		fetched, alertErr := s.fetchAlerts(ctx)
		switch {
		case alertErr == nil:
			alerts = fetched
		case isBackoffable(alertErr):
			s.logger.Warn("trains: realtime alert poll backing off", "error", alertErr)
		default:
			return alertErr
		}
	}

	s.mu.Lock()
	s.snapshot = models.Snapshot{
		Trips:     trips,
		Alerts:    alerts,
		FetchedAt: time.Now(),
	}
	listeners := make([]func(), len(s.onUpdate))
	copy(listeners, s.onUpdate)
	s.mu.Unlock()

	for _, fn := range listeners {
		fn()
	}
	return nil
}

func (s *RealtimeService) fetchTripUpdates(
	ctx context.Context,
) (map[string]models.TripUpdate, error) {
	res, err := s.bmc.FetchRealtime(ctx, bmc.FeedTripUpdate)
	if err != nil {
		return nil, err
	}
	return decodeTripUpdates(res.Body)
}

func (s *RealtimeService) fetchAlerts(ctx context.Context) ([]models.Alert, error) {
	res, err := s.bmc.FetchRealtime(ctx, bmc.FeedAlert)
	if err != nil {
		return nil, err
	}
	return decodeAlerts(res.Body)
}

// isBackoffable reports whether err is a transient gateway condition the
// next scheduled poll should simply retry, rather than a bug worth failing
// the job over.
func isBackoffable(err error) bool {
	var rateLimited *bmc.RateLimitedError
	if errors.As(err, &rateLimited) {
		return true
	}
	var upstream *bmc.UpstreamError
	if errors.As(err, &upstream) {
		const serverErrorFloor = 500
		return upstream.StatusCode >= serverErrorFloor
	}
	return false
}
