package wstools

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/internal/validate"
)

// SubscribeMessageDto is implemented by all messages
// used to subscribe to a certain handler of a [WebSocketHandler].
type SubscribeMessageDto interface {
	validate.ValidatedType
	Topic() string
}

// A WebSocketHandler handles incoming requests to a
// websocket and makes sure subscriptions are made to the right topics.
type WebSocketHandler[T SubscribeMessageDto] struct {
	ctx                    context.Context
	logger                 *slog.Logger
	maxTopicWorkers        int
	topicChannelBufferSize int
	topicMap               map[string]*Topic
}

// CreateWebSocketHandler creates a new [WebSocketHandler].
func CreateWebSocketHandler[T SubscribeMessageDto](
	ctx context.Context,
	logger *slog.Logger,
	maxTopicWorkers int,
	topicChannelBufferSize int,
) WebSocketHandler[T] {
	return WebSocketHandler[T]{
		ctx:                    ctx,
		logger:                 logger,
		maxTopicWorkers:        maxTopicWorkers,
		topicChannelBufferSize: topicChannelBufferSize,
		topicMap:               make(map[string]*Topic),
	}
}

// AddTopic adds a topic to which can be subscribed using a [SubscribeMessageDto].
// The onSubscribeCallback is called for each
// new subscriber to fetch data to send them back.
func (h *WebSocketHandler[T]) AddTopic(
	topicName string,
	allowedOrigins []string,
	onSubscribeCallback OnSubscribeCallback,
) (*Topic, error) {
	_, ok := h.topicMap[topicName]
	if ok {
		return nil, fmt.Errorf("topic '%s' has already been added", topicName)
	}

	topic := NewTopic(
		h.ctx,
		h.logger,
		topicName,
		allowedOrigins,
		h.maxTopicWorkers,
		h.topicChannelBufferSize,
		onSubscribeCallback,
	)
	h.topicMap[topicName] = topic

	return topic, nil
}

// UpdateTopicName updates the name of a topic without losing its subscribers.
func (h *WebSocketHandler[T]) UpdateTopicName(
	topic *Topic,
	newName string,
) (*Topic, error) {
	newTopic, ok := h.topicMap[topic.Name]
	if !ok {
		return nil, fmt.Errorf("topic '%s' doesn't exist", topic.Name)
	}

	_, ok = h.topicMap[newName]
	if ok {
		return nil, fmt.Errorf("topic '%s' already exists", newName)
	}

	newTopic.Name = newName
	delete(h.topicMap, topic.Name)
	h.topicMap[newTopic.Name] = newTopic

	return newTopic, nil
}

// RemoveTopic removes a topic to which can be subscribed using a [SubscribeMessageDto].
func (h *WebSocketHandler[T]) RemoveTopic(topic *Topic) error {
	_, ok := h.topicMap[topic.Name]
	if !ok {
		return fmt.Errorf("topic '%s' doesn't exist", topic.Name)
	}

	delete(h.topicMap, topic.Name)
	return nil
}

// Handler returns the [http.HandlerFunc] of a [WebSocketHandler].
func (h WebSocketHandler[T]) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := acceptWithHandshakeSpan(w, r)
		if err != nil {
			UpgradeErrorResponse(w, r, err)
			return
		}

		// in case you want to subscribe on multiple topics
		for {
			var msg T
			err = wsjson.Read(r.Context(), conn, &msg)
			if err != nil {
				ServerErrorResponse(r.Context(), conn, err)
				return
			}

			if valid, errors := msg.Validate(); !valid {
				FailedValidationResponse(r.Context(), conn, errors)
				return
			}

			topic, ok := h.topicMap[msg.Topic()]
			if !ok {
				ErrorResponse(
					r.Context(),
					conn,
					http.StatusBadRequest,
					fmt.Sprintf("topic '%s' doesn't exist", msg.Topic()),
				)
				return
			}

			err = authenticateOrigin(r, topic.allowedOrigins)
			if err != nil {
				ForbiddenResponse(r.Context(), conn)
			}

			err = topic.Subscribe(conn)
			if err != nil {
				ServerErrorResponse(r.Context(), conn, err)
				return
			}
		}
	}
}

// acceptWithHandshakeSpan performs the WebSocket handshake/upgrade,
// measuring it as its own Sentry transaction rather than letting it be
// folded into the connection-lifetime transaction sentryhttp's middleware
// starts around the whole handler call. That outer transaction never
// returns until the socket closes, so its "duration" measures how long a
// client kept the connection open, not request latency — a WebSocket route
// is excluded from HTTP-class slow-transaction alerting entirely for that
// reason (isWebSocketProgressTransaction,
// internal/observability/jobs/threshold_alert_slow_transactions.go, issue
// #1320). Excluding it from alerting is necessary (that duration can never
// be a meaningful latency signal) but not sufficient on its own — this
// gives the same route a bounded, comparable metric instead: how long the
// actual handshake took, still subject to the ordinary HTTP threshold, so a
// real upgrade-path regression (e.g. a slow auth check added in front of
// websocket.Accept) still alerts.
//
// The transaction is started from a fresh context rather than r.Context():
// sentry.StartTransaction returns the existing transaction unchanged when
// one is already present in the context (which r.Context() carries, from
// sentryhttp) instead of starting a new one, so reusing r.Context() here
// would just hand back that same connection-lifetime transaction. The name
// carries a distinct "[ws-handshake]" suffix so it aggregates in Sentry
// separately from the connection transaction's own "GET .../api/progress"
// name (same method+path prefix, so it still classifies as an HTTP handler)
// and doesn't match isWebSocketProgressTransaction's exclusion.
func acceptWithHandshakeSpan(
	w http.ResponseWriter,
	r *http.Request,
) (*websocket.Conn, error) {
	hub := sentry.GetHubFromContext(r.Context())
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	txnCtx := sentry.SetHubOnContext(context.Background(), hub)
	txn := sentry.StartTransaction(
		txnCtx,
		fmt.Sprintf("%s %s [ws-handshake]", r.Method, r.URL.Path),
		sentry.WithOpName("websocket.server"),
	)
	defer txn.Finish()

	//nolint:exhaustruct //other fields are optional
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		txn.Status = sentry.SpanStatusInternalError
		return nil, err
	}
	txn.Status = sentry.SpanStatusOK
	return conn, nil
}

// copied from github.com/coder/websocket.
func authenticateOrigin(r *http.Request, originHosts []string) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}

	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("failed to parse Origin header %q: %w", origin, err)
	}

	if strings.EqualFold(r.Host, u.Host) {
		return nil
	}

	for _, hostPattern := range originHosts {
		var matched bool
		matched, err = match(hostPattern, u.Host)
		if err != nil {
			return fmt.Errorf(
				"failed to parse filepath pattern %q: %w",
				hostPattern,
				err,
			)
		}
		if matched {
			return nil
		}
	}
	if u.Host == "" {
		return fmt.Errorf("request Origin %q is not a valid URL with a host", origin)
	}
	return fmt.Errorf("request Origin %q is not authorized for Host %q", u.Host, r.Host)
}

// copied from github.com/coder/websocket.
func match(pattern, s string) (bool, error) {
	return filepath.Match(strings.ToLower(pattern), strings.ToLower(s))
}
