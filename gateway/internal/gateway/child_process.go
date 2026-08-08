// Package gateway implements the merged single-container deploy shape
// (issue #558, later split into its own binary): it owns request routing
// between the api and web processes and supervises both as children.
package gateway

import (
	"log/slog"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
)

// childProcessStopTimeout bounds how long Stop waits for a clean exit before
// escalating to SIGKILL. A var (not const) so tests can shrink it instead of
// waiting out the real timeout.
//
//nolint:gochecknoglobals //test seam, see comment above
var childProcessStopTimeout = 10 * time.Second

// shutdownSignal asks this process to shut down after a child exits
// unexpectedly. A var (not a direct syscall.Kill call) so tests can observe
// it firing without actually killing the test binary.
//
//nolint:gochecknoglobals //test seam, see comment above
var shutdownSignal = func() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ChildProcess supervises one child process (api or web) that gateway
// front-doors with a reverse proxy (see proxy.go). Both children get
// identical supervision semantics: SIGTERM-then-SIGKILL on Stop, and an
// unexpected exit brings the whole gateway (and thus the container) down —
// serving requests to a dead upstream forever is worse than a restart.
type ChildProcess struct {
	name     string
	cmd      *exec.Cmd
	stopping atomic.Bool
	done     chan struct{}
}

// StartChildProcess starts cmd (the caller has already set its Env/Stdout/
// Stderr, and its Context if it needs to be tied to one via
// exec.CommandContext) and returns once it has started, not once it's ready
// to accept connections. name is used only for log messages.
func StartChildProcess(
	logger *slog.Logger, name string, cmd *exec.Cmd,
) (*ChildProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	//nolint:exhaustruct //stopping zero-values to false, which is correct here
	c := &ChildProcess{name: name, cmd: cmd, done: make(chan struct{})}
	go c.waitAndSignalOnUnexpectedExit(logger)

	return c, nil
}

func (c *ChildProcess) waitAndSignalOnUnexpectedExit(logger *slog.Logger) {
	err := c.cmd.Wait()
	close(c.done)

	if c.stopping.Load() {
		return
	}

	logger.Error(
		c.name+" process exited unexpectedly", essentialogger.ErrAttr(err),
	)
	shutdownSignal()
}

// Stop terminates the child process, escalating to SIGKILL if it hasn't
// exited within childProcessStopTimeout.
func (c *ChildProcess) Stop() {
	c.stopping.Store(true)

	if c.cmd.Process == nil {
		return
	}

	_ = c.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-c.done:
	case <-time.After(childProcessStopTimeout):
		_ = c.cmd.Process.Kill()
		<-c.done
	}
}
