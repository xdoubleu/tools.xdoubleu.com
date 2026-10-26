package httptools_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/logging"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func TestServe_ReturnsOnlyAfterInFlightRequestsDrain(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{})
	release := make(chan struct{})
	addr := freeAddr(t)

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			_, _ = io.WriteString(w, "done")
		}),
		ReadHeaderTimeout: time.Second,
	}

	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() {
		served <- httptools.Serve(ctx, logging.NewNopLogger(), srv, "test", time.Minute)
	}()

	body := make(chan string, 1)
	go func() {
		var res *http.Response
		var err error
		for range 100 {
			res, err = http.Get("http://" + addr)
			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !assert.NoError(t, err) {
			body <- ""
			return
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		body <- string(b)
	}()

	<-entered
	cancel()

	select {
	case err := <-served:
		t.Fatalf("Serve returned before the request drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	assert.Equal(t, "done", <-body)
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve never returned after the request drained")
	}
}

func TestServe_ShutdownTimeoutClosesStuckConnections(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{})
	addr := freeAddr(t)

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(entered)
			<-r.Context().Done()
		}),
		ReadHeaderTimeout: time.Second,
	}

	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() {
		served <- httptools.Serve(
			ctx, logging.NewNopLogger(), srv, "test", 50*time.Millisecond,
		)
	}()

	go func() {
		for range 100 {
			res, err := http.Get("http://" + addr)
			if err == nil {
				_ = res.Body.Close()
				return
			}
			select {
			case <-entered:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()

	<-entered
	cancel()

	select {
	case err := <-served:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve never gave up on the stuck request")
	}
}

func TestServe_ReturnsListenError(t *testing.T) {
	t.Parallel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	srv := &http.Server{Addr: l.Addr().String(), ReadHeaderTimeout: time.Second}
	err = httptools.Serve(t.Context(), logging.NewNopLogger(), srv, "test", time.Second)
	require.Error(t, err)
}
