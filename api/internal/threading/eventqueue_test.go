package threading_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/threading"
)

type fakeSubscriber struct {
	id       string
	received chan any
}

func (s *fakeSubscriber) ID() string { return s.id }

func (s *fakeSubscriber) OnEventCallback(event any) {
	s.received <- event
}

func TestEventQueueDeliversToSubscribers(t *testing.T) {
	t.Parallel()

	q := threading.NewEventQueue(context.Background(), logging.NewNopLogger(), 1, 1)

	sub := &fakeSubscriber{id: "sub-1", received: make(chan any, 1)}
	q.AddSubscriber(sub)

	q.EnqueueEvent("hello")

	select {
	case event := <-sub.received:
		assert.Equal(t, "hello", event)
	case <-time.After(time.Second):
		t.Fatal("event was never delivered")
	}

	q.RemoveSubscriber(sub)
	q.EnqueueEvent("world")

	select {
	case event := <-sub.received:
		t.Fatalf("unexpected event after unsubscribe: %v", event)
	case <-time.After(100 * time.Millisecond):
	}
}
