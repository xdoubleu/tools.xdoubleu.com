package digitalocean_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/digitalocean"
)

// liveHandler serves DigitalOcean's live log socket: one JSON frame per
// chunk, then a normal close — what DO does once it has replayed tail_lines
// with follow off.
func liveHandler(chunks ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		for _, chunk := range chunks {
			frame, _ := json.Marshal(map[string]string{"data": chunk})
			if writeErr := conn.Write(
				r.Context(), websocket.MessageText, frame,
			); writeErr != nil {
				return
			}
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
}

// rawLiveHandler sends frames verbatim, without DO's {"data": …} envelope.
func rawLiveHandler(chunks ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		for _, chunk := range chunks {
			if writeErr := conn.Write(
				r.Context(), websocket.MessageText, []byte(chunk),
			); writeErr != nil {
				return
			}
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
}

// newLiveMux builds a fake DigitalOcean server whose dep-1 is the active
// deployment and whose component/type pairs are served over a live socket.
func newLiveMux(
	components []string,
	live map[string]string,
	chunkKeys map[string][]string,
) *http.ServeMux {
	//nolint:exhaustruct // the app-scoped endpoints are exercised elsewhere
	return (&doServer{
		components: components,
		chunks:     chunkKeys,
		live:       live,
		activeID:   "dep-1",
	}).mux()
}

// A running component has nothing archived yet, so live_url is the only
// source of its runtime output — this is the case issue #632 was about.
func TestDeploymentLogs_LiveOnlyComponent(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		nil,
	)
	mux.HandleFunc("/live/api-run", liveHandler("serving :8000\n", "GET / 200\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, digitalocean.LogTypeRun, logs[0].Type)
	assert.Equal(t, "serving :8000\nGET / 200\n", logs[0].Content)
	assert.False(t, logs[0].Truncated)
}

func TestDeploymentLogs_AppendsLiveAfterHistoric(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		map[string][]string{"api:RUN": {"archived"}},
	)
	mux.HandleFunc("/log-chunk/archived", textHandler("rotated line\n"))
	mux.HandleFunc("/live/api-run", liveHandler("fresh line\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "rotated line\nfresh line\n", logs[0].Content)
}

// A frame that isn't DO's envelope is passed through rather than dropped, so
// a JSON-formatted log line survives.
func TestDeploymentLogs_LivePassesThroughUnenvelopedFrames(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		nil,
	)
	mux.HandleFunc("/live/api-run", rawLiveHandler(`{"level":"info"}`, "plain\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, `{"level":"info"}plain`+"\n", logs[0].Content)
}

func TestDeploymentLogs_LiveTruncatesAtCap(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		map[string][]string{"api:RUN": {"archived"}},
	)
	mux.HandleFunc("/log-chunk/archived", textHandler(strings.Repeat("h", 100*1024)))
	mux.HandleFunc("/live/api-run", liveHandler(strings.Repeat("l", 150*1024)))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.True(t, logs[0].Truncated)
	assert.Len(t, logs[0].Content, 200*1024)
}

// A socket that can't be reached must not cost the caller the build/deploy
// logs it already has.
func TestDeploymentLogs_LiveDialFailureDegrades(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		map[string][]string{"api:BUILD": {"build"}},
	)
	mux.HandleFunc("/log-chunk/build", textHandler("built\n"))
	mux.HandleFunc("/live/api-run", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, digitalocean.LogTypeBuild, logs[0].Type)
	assert.Equal(t, "built\n", logs[0].Content)
}

// An abrupt close mid-stream keeps what was read and flags it truncated.
func TestDeploymentLogs_LiveAbruptCloseTruncates(t *testing.T) {
	mux := newLiveMux(
		[]string{"api"},
		map[string]string{"api:RUN": "/live/api-run"},
		nil,
	)
	mux.HandleFunc("/live/api-run", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		frame, _ := json.Marshal(map[string]string{"data": "partial\n"})
		_ = conn.Write(r.Context(), websocket.MessageText, frame)
		_ = conn.CloseNow() // no close handshake
	})
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "partial\n", logs[0].Content)
	assert.True(t, logs[0].Truncated)
}
