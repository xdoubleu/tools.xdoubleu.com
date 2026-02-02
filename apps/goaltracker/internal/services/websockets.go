package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	wstools "github.com/xdoubleu/essentia/v2/pkg/communication/wstools"
	"github.com/xdoubleu/essentia/v2/pkg/threading"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
)

type WebSocketService struct {
	allowedOrigins []string
	handler        *wstools.WebSocketHandler[dtos.SubscribeMessageDto]
	jobQueue       *threading.JobQueue
	topics         map[string]*wstools.Topic
}

func NewWebSocketService(
	logger *slog.Logger,
	allowedOrigins []string,
	jobQueue *threading.JobQueue,
) *WebSocketService {
	service := WebSocketService{
		allowedOrigins: allowedOrigins,
		handler:        nil,
		jobQueue:       jobQueue,
		topics:         make(map[string]*wstools.Topic),
	}

	handler := wstools.CreateWebSocketHandler[dtos.SubscribeMessageDto](
		logger,
		1,
		100, //nolint:mnd //no magic number
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
	topic, ok := service.topics[id]
	if !ok {
		return
	}

	topic.EnqueueEvent(dtos.StateMessageDto{
		IsRefreshing: isRunning,
		LastRefresh:  lastRunTime,
	})
}

func (service *WebSocketService) RegisterTopics(topics []string) {
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
	}
}
