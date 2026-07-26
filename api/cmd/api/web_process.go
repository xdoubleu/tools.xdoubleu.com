package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/config"
)

// webProcessStopTimeout bounds how long Stop waits for a clean exit before
// escalating to SIGKILL. A var (not const), mirroring migrationLockTimeout
// in main.go, so tests can shrink it instead of waiting out the real timeout.
//
//nolint:gochecknoglobals //test seam, see comment above
var webProcessStopTimeout = 10 * time.Second

// shutdownSignal asks this process to shut down after the web child exits
// unexpectedly. A var (not a direct syscall.Kill call), mirroring
// migrationLockTimeout in main.go, so tests can observe it firing without
// actually killing the test binary.
//
//nolint:gochecknoglobals //test seam, see comment above
var shutdownSignal = func() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// webProcess supervises the Next.js standalone server child process that the
// merged single-container deploy shape (issue #558) runs behind this
// binary's reverse proxy (see frontend_proxy.go).
type webProcess struct {
	cmd      *exec.Cmd
	stopping atomic.Bool
	done     chan struct{}
}

// startWebProcess launches cfg.WebServerJS with cfg.WebNodeBin. It returns
// once the process has started, not once it's ready to accept connections.
func startWebProcess(
	ctx context.Context, logger *slog.Logger, cfg config.Config,
) (*webProcess, error) {
	//nolint:gosec // WebNodeBin/WebServerJS are server-owned config, not user input
	cmd := exec.CommandContext(ctx, cfg.WebNodeBin, cfg.WebServerJS)
	cmd.Env = webProcessEnv(cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	//nolint:exhaustruct //stopping zero-values to false, which is correct here
	w := &webProcess{cmd: cmd, done: make(chan struct{})}
	go w.waitAndSignalOnUnexpectedExit(logger)

	return w, nil
}

// webProcessEnv builds an explicit allowlist for the child's environment
// rather than passing os.Environ() wholesale. Two things are load-bearing:
// the parent's own PORT (config.Config reads PORT too) must never leak to
// the child, and RELEASE must always reach it — without it getRelease()
// returns 'dev' in the browser and gatewayNeedsUpdate
// (web/lib/reading/gatewayClient.ts) silently stops detecting installed
// kobo-gateway updates for every user.
func webProcessEnv(cfg config.Config) []string {
	return []string{
		"PORT=" + strconv.Itoa(cfg.WebPort),
		"HOSTNAME=127.0.0.1",
		"NODE_ENV=production",
		"NODE_OPTIONS=--max-old-space-size=192",
		"API_URL=" + cfg.APIURL,
		"SENTRY_DSN=" + cfg.SentryDsn,
		"SUPABASE_URL=" + cfg.SupabaseURL,
		"SUPABASE_ANON_KEY=" + cfg.SupabaseAnonKey,
		"RELEASE=" + cfg.Release,
	}
}

// waitAndSignalOnUnexpectedExit blocks until the child exits. If that
// happens while Stop hasn't been called, it logs at Error level (so it
// reaches Sentry) and asks this process to shut down — serving every page
// request with a dead upstream forever is worse than a container restart.
func (w *webProcess) waitAndSignalOnUnexpectedExit(logger *slog.Logger) {
	err := w.cmd.Wait()
	close(w.done)

	if w.stopping.Load() {
		return
	}

	logger.Error("web process exited unexpectedly", essentialogger.ErrAttr(err))
	shutdownSignal()
}

// Stop terminates the child process, escalating to SIGKILL if it hasn't
// exited within webProcessStopTimeout.
func (w *webProcess) Stop() {
	w.stopping.Store(true)

	if w.cmd.Process == nil {
		return
	}

	_ = w.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-w.done:
	case <-time.After(webProcessStopTimeout):
		_ = w.cmd.Process.Kill()
		<-w.done
	}
}
