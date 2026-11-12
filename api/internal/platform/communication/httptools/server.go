package httptools

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Serve runs srv until ctx is done, then shuts it down and returns once
// in-flight requests have drained. Connections still open after
// shutdownTimeout are closed.
func Serve(
	ctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	environment string,
	shutdownTimeout time.Duration,
) error {
	logger.Info(
		"starting server",
		slog.String("env", environment),
		slog.String("addr", srv.Addr),
	)

	listened := make(chan error, 1)
	go func() { listened <- srv.ListenAndServe() }()

	select {
	case err := <-listened:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down server", slog.String("addr", srv.Addr))

	shutdownCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), shutdownTimeout,
	)
	defer cancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		err = errors.Join(err, srv.Close())
	}
	if listenErr := <-listened; !errors.Is(listenErr, http.ErrServerClosed) {
		err = errors.Join(err, listenErr)
	}

	logger.Info("stopped server", slog.String("addr", srv.Addr))

	return err
}
