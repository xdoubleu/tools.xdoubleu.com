package wstools

import (
	"context"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

// Subscriber receives messages from a [Topic] over a [websocket.Conn].
type Subscriber struct {
	id    string
	ctx   context.Context
	topic *Topic
	conn  *websocket.Conn
}

// NewSubscriber returns a new [Subscriber].
func NewSubscriber(topic *Topic, conn *websocket.Conn) Subscriber {
	return Subscriber{
		id:    uuid.NewString(),
		ctx:   context.Background(),
		topic: topic,
		conn:  conn,
	}
}

// ID returns the id of a [Subscriber].
func (sub Subscriber) ID() string {
	return sub.id
}

// OnEventCallback writes an event to the subscriber, unsubscribing it if the
// connection is closed.
func (sub Subscriber) OnEventCallback(event any) {
	err := wsjson.Write(sub.ctx, sub.conn, event)
	if err == nil {
		return
	}

	if websocket.CloseStatus(err) != -1 {
		sub.topic.UnSubscribe(sub)
		return
	}

	ServerErrorResponse(sub.ctx, sub.conn, err)
}
