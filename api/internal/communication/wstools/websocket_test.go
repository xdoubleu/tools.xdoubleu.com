package wstools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/wstools"
	"tools.xdoubleu.com/internal/errortools"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/validate"
)

type testResponse struct {
	Ok bool `json:"ok"`
}

type testSubscribeMsg struct {
	TopicName string `json:"topicName"`
}

func (s testSubscribeMsg) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "topicName", s.TopicName, validate.IsNotEmpty)
	return v.Valid(), v.Errors()
}

func (s testSubscribeMsg) Topic() string {
	return s.TopicName
}

// dialAndExchange dials handler over a websocket, writes initialMsg (if any)
// and decodes one JSON response into dst.
func dialAndExchange(t *testing.T, handler http.Handler, initialMsg any, dst any) {
	t.Helper()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck //best-effort cleanup, test is already done

	if initialMsg != nil {
		require.NoError(t, wsjson.Write(ctx, conn, initialMsg))
	}

	require.NoError(t, wsjson.Read(ctx, conn, dst))
}

func TestWebSocketExistingTopic(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	_, err := ws.AddTopic("exists", []string{"http://localhost"},
		func(_ context.Context, _ *wstools.Topic) (any, error) {
			return testResponse{Ok: true}, nil
		})
	require.NoError(t, err)

	var got testResponse
	dialAndExchange(t, ws.Handler(), testSubscribeMsg{TopicName: "exists"}, &got)
	assert.True(t, got.Ok)
}

func TestWebSocketUnknownTopic(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)

	var got errortools.ErrorDto
	dialAndExchange(t, ws.Handler(), testSubscribeMsg{TopicName: "unknown"}, &got)
	assert.Equal(t, http.StatusText(http.StatusBadRequest), got.Error)
	assert.Equal(t, "topic 'unknown' doesn't exist", got.Message)
}

func TestWebSocketInvalidMessage(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)

	var got errortools.ErrorDto
	dialAndExchange(t, ws.Handler(), testSubscribeMsg{TopicName: ""}, &got)
	assert.Equal(t, http.StatusText(http.StatusUnprocessableEntity), got.Error)
}

func TestAddTopicDuplicate(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	topic, err := ws.AddTopic("exists", []string{"http://localhost"}, nil)
	require.NoError(t, err)
	require.NotNil(t, topic)

	topic, err = ws.AddTopic("exists", []string{"http://localhost"}, nil)
	assert.Nil(t, topic)
	assert.EqualError(t, err, "topic 'exists' has already been added")
}

func TestUpdateAndRemoveTopic(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	topic, err := ws.AddTopic("exists", []string{"http://localhost"},
		func(_ context.Context, _ *wstools.Topic) (any, error) {
			return testResponse{Ok: true}, nil
		})
	require.NoError(t, err)

	renamed, err := ws.UpdateTopicName(topic, "renamed")
	require.NoError(t, err)
	require.NotNil(t, renamed)

	var got testResponse
	dialAndExchange(t, ws.Handler(), testSubscribeMsg{TopicName: "renamed"}, &got)
	assert.True(t, got.Ok)

	assert.NoError(t, ws.RemoveTopic(renamed))
}

func TestUpdateTopicNameConflict(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	topic, err := ws.AddTopic("exists", []string{"http://localhost"}, nil)
	require.NoError(t, err)

	renamed, err := ws.UpdateTopicName(topic, "exists")
	assert.Nil(t, renamed)
	assert.ErrorContains(t, err, "topic 'exists' already exists")
}

func TestUpdateUnknownTopic(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	renamed, err := ws.UpdateTopicName(&wstools.Topic{Name: "unknown"}, "new")
	assert.Nil(t, renamed)
	assert.ErrorContains(t, err, "topic 'unknown' doesn't exist")
}

func TestRemoveUnknownTopic(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	err := ws.RemoveTopic(&wstools.Topic{Name: "unknown"})
	assert.ErrorContains(t, err, "topic 'unknown' doesn't exist")
}

func TestForbiddenOrigin(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	_, err := ws.AddTopic("exists", []string{"allowed.example.com"}, nil)
	require.NoError(t, err)

	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http"),
		//nolint:exhaustruct //other fields are optional
		&websocket.DialOptions{
			HTTPHeader: http.Header{"Origin": {"http://not-allowed.example.com"}},
		})
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck //best-effort cleanup, test is already done

	require.NoError(t, wsjson.Write(ctx, conn, testSubscribeMsg{TopicName: "exists"}))

	var got errortools.ErrorDto
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	assert.Equal(t, http.StatusText(http.StatusForbidden), got.Error)
}
