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

// tripResolver maps raw GTFS-RT trip_ids to their static trips.trip_short_name
// — RealtimeService.Poll re-keys the snapshot through it so a live journey
// never joins to realtime data by a trip_id the two feeds only coincidentally
// agree on (issue #1484).
type tripResolver interface {
	ShortNamesByTripIDs(
		ctx context.Context,
		tripIDs []string,
	) (map[string]string, error)
}

// RealtimeService polls the BMC gateway's GTFS-Realtime feeds and keeps the
// latest decoded state in memory. The snapshot is wholly replaced on every
// poll and nothing here is persisted — a later slice decides what, if
// anything, is worth writing down (issue #1393).
type RealtimeService struct {
	logger   *slog.Logger
	bmc      bmc.Client
	resolver tripResolver

	mu        sync.RWMutex
	snapshot  models.Snapshot
	pollCount int
	onUpdate  []func()
}

func NewRealtimeService(
	logger *slog.Logger, bmcClient bmc.Client, resolver tripResolver,
) *RealtimeService {
	//nolint:exhaustruct //snapshot/pollCount/onUpdate start zero-valued
	return &RealtimeService{logger: logger, bmc: bmcClient, resolver: resolver}
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

	rawTrips, err := s.fetchTripUpdates(ctx)
	if err != nil {
		if isBackoffable(err) {
			s.logger.Warn("trains: realtime trip-update poll backing off", "error", err)
			return nil
		}
		return err
	}

	trips, unresolved, err := s.resolveTripUpdates(ctx, rawTrips)
	if err != nil {
		return err
	}
	if unresolved > 0 {
		s.logger.Warn(
			"trains: realtime trip updates without a matching static trip",
			"unresolved", unresolved,
			"total", len(rawTrips),
		)
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
		Trips:               trips,
		UnresolvedTripCount: unresolved,
		Alerts:              alerts,
		FetchedAt:           time.Now(),
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

// correlateTripUpdates re-keys raw, trip_id-keyed trip updates off
// (trip_short_name, service date) by resolving each trip_id against the
// current static import. Updates whose trip_id has no static match are
// dropped and counted — the return's second value — so a growing gap between
// the two feeds' trip_id namespaces shows up as a metric rather than as
// silently missing delays (issue #1484).
func (s *RealtimeService) resolveTripUpdates(
	ctx context.Context, raw map[string]models.TripUpdate,
) (map[models.TripKey]models.TripUpdate, int, error) {
	ids := make([]string, 0, len(raw))
	for id := range raw {
		ids = append(ids, id)
	}
	shortNames, err := s.resolver.ShortNamesByTripIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	fallbackDate := time.Now().In(brusselsLoc).Format("20060102")
	trips, unresolved := correlateTripUpdates(raw, shortNames, fallbackDate)
	return trips, unresolved, nil
}

// correlateTripUpdates is the pure core of RealtimeService.resolveTripUpdates:
// given the trip_id→trip_short_name resolution and a fallback service date for
// updates the feed left undated, it produces the (trip_short_name, date)-keyed
// map and the count it could not place.
func correlateTripUpdates(
	raw map[string]models.TripUpdate,
	shortNames map[string]string,
	fallbackDate string,
) (map[models.TripKey]models.TripUpdate, int) {
	trips := make(map[models.TripKey]models.TripUpdate, len(raw))
	unresolved := 0
	for tripID, tu := range raw {
		shortName, ok := shortNames[tripID]
		if !ok {
			unresolved++
			continue
		}
		date := tu.StartDate
		if date == "" {
			date = fallbackDate
		}
		trips[models.TripKey{ShortName: shortName, Date: date}] = tu
	}
	return trips, unresolved
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
