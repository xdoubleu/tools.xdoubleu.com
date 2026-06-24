package services

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	wstools "github.com/xdoubleu/essentia/v4/pkg/communication/wstools"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/backlog/internal/dtos"
)

type WebSocketService struct {
	allowedOrigins []string
	handler        *wstools.WebSocketHandler[dtos.SubscribeMessageDto]
	jobQueue       *threading.JobQueue
	mu             sync.RWMutex
	topics         map[string]*wstools.Topic
}

func NewWebSocketService(
	ctx context.Context,
	logger *slog.Logger,
	allowedOrigins []string,
	jobQueue *threading.JobQueue,
) *WebSocketService {
	service := WebSocketService{
		allowedOrigins: allowedOrigins,
		handler:        nil,
		jobQueue:       jobQueue,
		mu:             sync.RWMutex{},
		topics:         make(map[string]*wstools.Topic),
	}

	const topicWorkers = 1
	const topicQueueSize = 100
	handler := wstools.CreateWebSocketHandler[dtos.SubscribeMessageDto](
		ctx,
		logger,
		topicWorkers,
		topicQueueSize,
	)

	service.handler = &handler

	return &service
}

func (service *WebSocketService) Handler() http.HandlerFunc {
	return service.handler.Handler()
}

func (service *WebSocketService) UpdateState(
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

	topic.EnqueueEvent(dtos.StateMessageDto{
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
func (service *WebSocketService) UpdateProgress(id string, processed, total int) {
	service.mu.RLock()
	topic, ok := service.topics[id]
	service.mu.RUnlock()
	if !ok {
		return
	}

	topic.EnqueueEvent(dtos.StateMessageDto{
		IsRefreshing: true,
		LastRefresh:  nil,
		Processed:    &processed,
		Total:        &total,
	})
}

func (service *WebSocketService) ForceRun(id string) {
	service.jobQueue.ForceRun(id)
}

func (service *WebSocketService) RegisterTopics(topics []string) {
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

func (service *WebSocketService) fetchState(topic *wstools.Topic) dtos.StateMessageDto {
	isRefreshing, lastRefresh := service.jobQueue.FetchState(topic.Name)

	return dtos.StateMessageDto{
		IsRefreshing: isRefreshing,
		LastRefresh:  lastRefresh,
		Processed:    nil,
		Total:        nil,
	}
}
