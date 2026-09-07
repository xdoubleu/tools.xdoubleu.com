package services

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/internal/communication/wstools"
)

// journeyDetailFetcher is the slice of JourneyDetailService JourneyWSService
// needs — narrowed to a testable interface rather than the concrete type.
type journeyDetailFetcher interface {
	GetJourneyDetail(
		ctx context.Context,
		journeyID string,
	) (*models.JourneyDetail, error)
}

// journeySubscribeDto is the client -> server subscription message: the
// client names the topic (journey id) it wants live updates for — the same
// shape internal/progressws uses for its job-progress topics.
type journeySubscribeDto struct {
	Subject string `json:"subject"`
}

func (d journeySubscribeDto) Topic() string { return d.Subject }

func (d journeySubscribeDto) Validate() (bool, map[string]string) {
	if d.Subject == "" {
		return false, map[string]string{"subject": "is required"}
	}
	return true, map[string]string{}
}

// JourneyWSService exposes a wstools topic per subscribed live journey
// (issue #1394). Unlike internal/progressws' fixed, up-front job-ID topics,
// journey topics are created lazily — GetJourneyDetail (the RPC a client
// calls before opening the socket) ensures one exists for the journey id it
// was asked about via EnsureTopic — and pushed a fresh snapshot after every
// realtime poll cycle via PushAll, so an open page gets updates without
// re-asking.
//
// Topics are never removed once created — a journey stops being pushed to
// only when the process itself restarts. That is an acceptable bound for a
// single long-lived server process at this feature's scale; a follow-up can
// add expiry (e.g. once a leg's arrival time is comfortably in the past)
// once journey volume warrants it.
type JourneyWSService struct {
	logger         *slog.Logger
	detail         journeyDetailFetcher
	handler        wstools.WebSocketHandler[journeySubscribeDto]
	allowedOrigins []string

	mu     sync.RWMutex
	topics map[string]*wstools.Topic
}

func NewJourneyWSService(
	ctx context.Context,
	logger *slog.Logger,
	allowedOrigins []string,
	detail journeyDetailFetcher,
) *JourneyWSService {
	const topicWorkers = 1
	const topicQueueSize = 20
	handler := wstools.CreateWebSocketHandler[journeySubscribeDto](
		ctx, logger, topicWorkers, topicQueueSize,
	)
	//nolint:exhaustruct //mu is a sync.RWMutex, zero-value ready to use
	return &JourneyWSService{
		logger:         logger,
		detail:         detail,
		handler:        handler,
		allowedOrigins: allowedOrigins,
		topics:         make(map[string]*wstools.Topic),
	}
}

// Handler is the http.HandlerFunc that upgrades and handles subscriptions
// for every registered journey topic.
func (s *JourneyWSService) Handler() http.HandlerFunc {
	return s.handler.Handler()
}

// EnsureTopic registers a topic for journeyID if one doesn't already exist,
// so a client can always open the websocket right after fetching detail.
func (s *JourneyWSService) EnsureTopic(journeyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.topics[journeyID]; ok {
		return
	}

	topic, err := s.handler.AddTopic(
		journeyID,
		s.allowedOrigins,
		func(ctx context.Context, tp *wstools.Topic) (any, error) {
			return s.detail.GetJourneyDetail(ctx, tp.Name)
		},
	)
	if err != nil {
		s.logger.Error("trains: failed to register journey topic", "error", err)
		return
	}
	s.topics[journeyID] = topic
}

// PushAll recomputes and broadcasts fresh detail to every registered
// journey topic — called after each realtime poll cycle (issue #1394) so an
// open page is pushed the update rather than only receiving it on its next
// (re)subscribe.
func (s *JourneyWSService) PushAll(ctx context.Context) {
	s.mu.RLock()
	topics := make([]*wstools.Topic, 0, len(s.topics))
	for _, t := range s.topics {
		topics = append(topics, t)
	}
	s.mu.RUnlock()

	for _, t := range topics {
		detail, err := s.detail.GetJourneyDetail(ctx, t.Name)
		if err != nil {
			s.logger.Warn(
				"trains: failed to rebuild journey detail for push",
				"journeyId", t.Name, "error", err,
			)
			continue
		}
		t.EnqueueEvent(detail)
	}
}
