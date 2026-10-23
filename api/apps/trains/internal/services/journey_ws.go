package services

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/internal/communication/wstools"
)

// journeyDetailFetcher is the slice of JourneyDetailService JourneyWSService
// needs.
type journeyDetailFetcher interface {
	GetJourneyDetail(
		ctx context.Context,
		journeyID string,
	) (*models.JourneyDetail, error)
}

// journeySubscribeDto names the journey topic a client subscribes to.
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

// pushInterval is how often watched journeys are re-fetched from iRail.
const pushInterval = 30 * time.Second

// topicGrace is how long a journey's topic outlives its arrival time.
const topicGrace = time.Hour

// JourneyWSService serves a lazily-created wstools topic per live journey:
// GetJourneyDetail calls EnsureTopic, and Run broadcasts fresh detail on a
// timer. A topic is dropped once its journey has ended, so only journeys that
// can still change are polled.
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

// Handler upgrades and serves subscriptions for all journey topics.
func (s *JourneyWSService) Handler() http.HandlerFunc {
	return s.handler.Handler()
}

// EnsureTopic registers a topic for journeyID if missing.
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

// Run pushes on every tick until ctx is done.
func (s *JourneyWSService) Run(ctx context.Context) {
	ticker := time.NewTicker(pushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.PushAll(ctx)
		}
	}
}

// PushAll broadcasts fresh detail to every journey topic, first dropping the
// topics of journeys that ended more than topicGrace ago.
func (s *JourneyWSService) PushAll(ctx context.Context) {
	s.dropEnded()
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

func (s *JourneyWSService) dropEnded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, topic := range s.topics {
		refs, err := DecodeJourneyID(id)
		if err != nil || time.Since(refs[len(refs)-1].AlightTime) < topicGrace {
			continue
		}
		if rmErr := s.handler.RemoveTopic(topic); rmErr != nil {
			s.logger.Warn("trains: failed to remove journey topic", "error", rmErr)
		}
		delete(s.topics, id)
	}
}
