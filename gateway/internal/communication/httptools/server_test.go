package httptools_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/communication/httptools"
)

func TestServe(t *testing.T) {
	t.Parallel()

	srv := &http.Server{
		Addr:         "localhost:0",
		Handler:      http.NewServeMux(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, srv.Shutdown(context.Background()))
	}()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	err := httptools.Serve(logger, srv, "test")

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "starting server")
	assert.Contains(t, buf.String(), "stopped server")
}
