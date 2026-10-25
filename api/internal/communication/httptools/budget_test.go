package httptools_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/logging"
)

const (
	testWriteTimeout = 100 * time.Millisecond
	pastWriteTimeout = 3 * testWriteTimeout
)

// serveWithWriteTimeout serves h with a short WriteTimeout, as main.go does.
func serveWithWriteTimeout(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.Config.WriteTimeout = testWriteTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func lateWriter(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(pastWriteTimeout)
	_, _ = w.Write([]byte("ok"))
}

func get(t *testing.T, srv *httptest.Server, header http.Header) (string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, srv.URL, nil,
	)
	require.NoError(t, err)
	req.Header = header
	resp, err := srv.Client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

// A response written after WriteTimeout never reaches the client, and the
// server logs nothing.
func TestWriteTimeoutCutsLateResponse(t *testing.T) {
	t.Parallel()
	srv := serveWithWriteTimeout(t, http.HandlerFunc(lateWriter))

	_, err := get(t, srv, http.Header{})
	require.Error(t, err)
}

// Hijacking clears the connection's deadlines, so WebSockets outlive
// WriteTimeout.
func TestWriteTimeoutSparesHijackedConn(t *testing.T) {
	t.Parallel()
	srv := serveWithWriteTimeout(t, http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			conn, buf, err := http.NewResponseController(w).Hijack()
			if err != nil {
				return
			}
			defer conn.Close()
			time.Sleep(pastWriteTimeout)
			_, _ = buf.WriteString(
				"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok",
			)
			_ = buf.Flush()
		},
	))

	body, err := get(t, srv, http.Header{})
	require.NoError(t, err)
	assert.Equal(t, "ok", body)
}

func TestBudgetLongRouteOutlivesWriteTimeout(t *testing.T) {
	t.Parallel()
	always := func(*http.Request) bool { return true }
	mw := httptools.Budget(logging.NewNopLogger(), always)
	srv := serveWithWriteTimeout(t, mw(http.HandlerFunc(lateWriter)))

	body, err := get(t, srv, http.Header{})
	require.NoError(t, err)
	assert.Equal(t, "ok", body)
}

func TestBudgetDefaultRouteKeepsWriteTimeout(t *testing.T) {
	t.Parallel()
	never := func(*http.Request) bool { return false }
	mw := httptools.Budget(logging.NewNopLogger(), never)
	srv := serveWithWriteTimeout(t, mw(http.HandlerFunc(lateWriter)))

	_, err := get(t, srv, http.Header{})
	require.Error(t, err)
}

// deadlineOf runs one request through Budget and returns the handler's
// context deadline.
func deadlineOf(
	t *testing.T,
	long bool,
	header http.Header,
) (time.Duration, bool) {
	t.Helper()
	var (
		left time.Duration
		ok   bool
	)
	mw := httptools.Budget(
		logging.NewNopLogger(), func(*http.Request) bool { return long },
	)
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var deadline time.Time
		deadline, ok = r.Context().Deadline()
		left = time.Until(deadline)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header = header
	h.ServeHTTP(httptest.NewRecorder(), req)
	return left, ok
}

func TestBudgetSetsRequestDeadline(t *testing.T) {
	t.Parallel()
	left, ok := deadlineOf(t, false, http.Header{})
	require.True(t, ok)
	assert.InDelta(t, httptools.RequestBudget, left, float64(time.Second))
}

func TestBudgetSetsLongRequestDeadline(t *testing.T) {
	t.Parallel()
	left, ok := deadlineOf(t, true, http.Header{})
	require.True(t, ok)
	assert.InDelta(t, httptools.LongRequestBudget, left, float64(time.Second))
}

// A WebSocket outlives any budget; its context must not expire.
func TestBudgetSkipsWebSocketUpgrade(t *testing.T) {
	t.Parallel()
	header := http.Header{}
	header.Set("Connection", "Upgrade")
	header.Set("Upgrade", "websocket")
	_, ok := deadlineOf(t, false, header)
	assert.False(t, ok)
}

func TestDetachKeepsDeadlineDropsCancel(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), time.Hour)
	want, _ := parent.Deadline()

	ctx, cancel := httptools.Detach(parent)
	defer cancel()
	cancelParent()

	require.NoError(t, ctx.Err())
	got, ok := ctx.Deadline()
	require.True(t, ok)
	assert.Equal(t, want, got)
}

func TestDetachWithoutDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := httptools.Detach(context.Background())
	defer cancel()

	_, ok := ctx.Deadline()
	assert.False(t, ok)
}
