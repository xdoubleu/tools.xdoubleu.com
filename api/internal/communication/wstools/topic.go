package wstools

import (
	"context"
	"log/slog"
	"strings"

	"github.com/coder/websocket"

	"tools.xdoubleu.com/internal/threading"
)

// OnSubscribeCallback returns the initial data sent to a new subscriber.
type OnSubscribeCallback = func(ctx context.Context, topic *Topic) (any, error)

// Topic fans messages out to its [Subscriber]s.
type Topic struct {
	Name                string
	allowedOrigins      []string
	eventQueue          *threading.EventQueue
	onSubscribeCallback OnSubscribeCallback
}

// NewTopic creates a new [Topic].
func NewTopic(
	ctx context.Context,
	logger *slog.Logger,
	name string,
	allowedOrigins []string,
	maxWorkers int,
	channelBufferSize int,
	onSubscribeCallback OnSubscribeCallback,
) *Topic {
	for i, url := range allowedOrigins {
		if strings.Contains(url, "://") {
			allowedOrigins[i] = strings.Split(url, "://")[1]
		}
	}

	return &Topic{
		Name:           name,
		allowedOrigins: allowedOrigins,
		eventQueue: threading.NewEventQueue(
			ctx,
			logger,
			maxWorkers,
			channelBufferSize,
		),
		onSubscribeCallback: onSubscribeCallback,
	}
}

// Subscribe adds a [Subscriber], sends the initial message if configured, and
// starts the message loop if it isn't running.
func (t *Topic) Subscribe(conn *websocket.Conn) error {
	sub := NewSubscriber(t, conn)
	t.eventQueue.AddSubscriber(sub)

	if t.onSubscribeCallback != nil {
		event, err := t.onSubscribeCallback(context.Background(), t)
		if err != nil {
			return err
		}

		sub.OnEventCallback(event)
	}

	return nil
}

// UnSubscribe unsubscribes a [Subscriber] from this [Topic].
func (t *Topic) UnSubscribe(sub Subscriber) {
	t.eventQueue.RemoveSubscriber(sub)
}

// EnqueueEvent enqueues an event if there are subscribers on this [Topic].
func (t *Topic) EnqueueEvent(event any) {
	t.eventQueue.EnqueueEvent(event)
}
