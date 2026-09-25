package services

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// alertPollEvery: alerts change rarely, so poll them every 4th cycle (~2min).
const alertPollEvery = 4

// tripResolver maps GTFS-RT trip_ids to static trip_short_names, so realtime
// data never joins by a trip_id the feeds only coincidentally share.
type tripResolver interface {
	ShortNamesByTripIDs(
		ctx context.Context,
		tripIDs []string,
	) (map[string]string, error)
}

// RealtimeService polls the BMC GTFS-Realtime feeds and keeps the latest
// state in memory, replaced wholesale each poll.
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

// OnUpdate registers fn to run after every successful Poll.
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

// Poll fetches trip updates every call and alerts less often, then replaces
// the snapshot. Transient gateway failures (see isBackoffable) skip this poll;
// the next run is the retry. Other errors are returned so they surface.
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
		if isBackoffable(err) {
			s.logger.Warn(
				"trains: realtime trip-update resolve backing off",
				"error",
				err,
			)
			return nil
		}
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

// resolveTripUpdates re-keys trip updates by (trip_short_name, service date),
// returning the count with no static match as a drift metric.
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

// correlateTripUpdates is resolveTripUpdates' pure core; fallbackDate dates
// updates the feed left undated.
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

// isBackoffable reports whether err is transient and the next poll should
// just retry: rate limits, 5xx, network timeouts, non-protobuf 200s, and a
// bare context.DeadlineExceeded (pgx's form of a query cut off by pollTimeout).
func isBackoffable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var rateLimited *bmc.RateLimitedError
	if errors.As(err, &rateLimited) {
		return true
	}
	var upstream *bmc.UpstreamError
	if errors.As(err, &upstream) {
		const serverErrorFloor = 500
		return upstream.StatusCode >= serverErrorFloor
	}
	// A non-protobuf 200 is the gateway serving an error page; transient.
	var badContentType *bmc.UnexpectedContentTypeError
	if errors.As(err, &badContentType) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
