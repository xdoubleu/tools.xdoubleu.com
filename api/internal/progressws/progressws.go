// Package progressws provides the WebSocket service used by apps to broadcast
// background-job progress (start/stop state and live "X of N" counts) to
// connected clients, keyed by job-ID topics.
package progressws

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	wstools "github.com/xdoubleu/essentia/v4/pkg/communication/wstools"
	"github.com/xdoubleu/essentia/v4/pkg/threading"
)

// SubscribeMessageDto is the client → server subscription message: the client
// names the topic (job ID) it wants state updates for.
type SubscribeMessageDto struct {
	Subject string `json:"subject"`
}

// StateMessageDto is the server → client state broadcast.
type StateMessageDto struct {
	LastRefresh  *time.Time `json:"lastRefresh"`
	IsRefreshing bool       `json:"isRefreshing"`
	// Processed and Total are set during long-running jobs (e.g. Open Library
	// resync) to give clients a live "X of N" count. They are omitted for jobs
	// that emit only start/stop events (e.g. Steam refresh).
	Processed *int `json:"processed,omitempty"`
	Total     *int `json:"total,omitempty"`
}

func (dto SubscribeMessageDto) Topic() string {
	return dto.Subject
}

func (dto SubscribeMessageDto) Validate() (bool, map[string]string) {
	return true, make(map[string]string)
}

type Service struct {
	allowedOrigins []string
	handler        *wstools.WebSocketHandler[SubscribeMessageDto]
	jobQueue       *threading.JobQueue
	mu             sync.RWMutex
	topics         map[string]*wstools.Topic
}

func NewService(
	ctx context.Context,
	logger *slog.Logger,
	allowedOrigins []string,
	jobQueue *threading.JobQueue,
) *Service {
	service := Service{
		allowedOrigins: allowedOrigins,
		handler:        nil,
		jobQueue:       jobQueue,
		mu:             sync.RWMutex{},
		topics:         make(map[string]*wstools.Topic),
	}

	const topicWorkers = 1
	const topicQueueSize = 100
	handler := wstools.CreateWebSocketHandler[SubscribeMessageDto](
		ctx,
		logger,
		topicWorkers,
		topicQueueSize,
	)

	service.handler = &handler

	return &service
}

func (service *Service) Handler() http.HandlerFunc {
	return service.handler.Handler()
}

func (service *Service) UpdateState(
	id string,
	isRunning bool,
	lastRunTime *time.Time,
) {
	service.mu.RLock()
	topic, ok := service.topics[id]
	service.mu.RUnlock()
	if !ok {
		return
	}

	topic.EnqueueEvent(StateMessageDto{
		IsRefreshing: isRunning,
		LastRefresh:  lastRunTime,
		Processed:    nil,
		Total:        nil,
	})
}

// UpdateProgress enqueues a mid-run progress event on the named topic. It is
// meant for long-running background jobs (e.g. the Open Library resync) that
// want to broadcast "X of N items done" to connected clients. The message
// carries IsRefreshing: true so clients keep the running indicator active.
func (service *Service) UpdateProgress(
	id string,
	processed, total int,
) {
	service.mu.RLock()
	topic, ok := service.topics[id]
	service.mu.RUnlock()
	if !ok {
		return
	}

	topic.EnqueueEvent(StateMessageDto{
		IsRefreshing: true,
		LastRefresh:  nil,
		Processed:    &processed,
		Total:        &total,
	})
}

func (service *Service) ForceRun(id string) {
	service.jobQueue.ForceRun(id)
}

func (service *Service) RegisterTopics(topics []string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	for _, topic := range topics {
		registeredTopic, err := service.handler.AddTopic(
			topic,
			service.allowedOrigins,
			func(_ context.Context, tp *wstools.Topic) (any, error) {
				return service.fetchState(tp), nil
			},
		)
		if err != nil {
			panic(err)
		}
		service.topics[topic] = registeredTopic
	}
}

func (service *Service) fetchState(topic *wstools.Topic) StateMessageDto {
	isRefreshing, lastRefresh := service.jobQueue.FetchState(topic.Name)

	return StateMessageDto{
		IsRefreshing: isRefreshing,
		LastRefresh:  lastRefresh,
		Processed:    nil,
		Total:        nil,
	}
}
