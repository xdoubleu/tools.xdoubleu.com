package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/gateway/internal/communication/httptools"
	"tools.xdoubleu.com/gateway/internal/gateway"
	"tools.xdoubleu.com/gateway/internal/logging"
	"tools.xdoubleu.com/gateway/internal/sentrytools"
)

const (
	httpReadTimeout  = 5 * time.Second
	httpWriteTimeout = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := gateway.New(logger)

	logger = slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	slog.SetDefault(logger)

	ctx := context.Background()

	sentryHub, err := sentrytools.Init(cfg.Env, sentry.ClientOptions{
		Dsn:         cfg.SentryDsn,
		Environment: cfg.Env,
		Release:     cfg.Release,
	})
	if err != nil {
		panic(err)
	}
	if sentryHub != nil {
		ctx = sentry.SetHubOnContext(ctx, sentryHub)
	}

	api, err := gateway.StartChildProcess(logger, "api", gateway.NewAPICmd(ctx, cfg))
	if err != nil {
		panic(err)
	}
	defer api.Stop()

	web, err := gateway.StartChildProcess(logger, "web", gateway.NewWebCmd(ctx, cfg))
	if err != nil {
		panic(err)
	}
	defer web.Stop()

	handler := gateway.NewHandler(cfg.APIPort, cfg.WebPort, cfg.Release, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}
	err = httptools.Serve(logger, srv, cfg.Env)
	if err != nil {
		logger.Error("failed to serve gateway", logging.ErrAttr(err))
	}
}
