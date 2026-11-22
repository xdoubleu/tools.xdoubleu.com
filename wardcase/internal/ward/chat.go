package ward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/vm"
)

// closeGrace is how long a harness gets to exit once its stdin closes, before the
// VM's exec is cancelled.
const closeGrace = 5 * time.Second

// ChatRequest is one interactive session over ACP.
type ChatRequest struct {
	// Source, when set, is cloned into a worktree on its own branch; otherwise the
	// session gets a worktree of the checkout containing Dir.
	Source *Source
	Dir    string
	// Harness must have an ACP server (vm.Harness.ACP).
	Harness string
	// OpenCodeModel overrides the config's OpenRouter model id when set.
	OpenCodeModel string
	// OnEvent receives each session/update, and each permission request wardcase
	// allowed. It is called one event at a time.
	OnEvent func(acp.Event)
}

// Chat is a running ACP session in its own VM and worktree.
type Chat struct {
	r         *Runner
	sess      session
	box       vm.SandboxSession
	client    *acp.Client
	id        string
	outcome   string
	stdin     *io.PipeWriter
	stop      context.CancelFunc
	attached  chan error
	closeOnce sync.Once
	left      Leftover
	removed   bool
}

// OpenChat starts the worktree and VM, runs the harness's ACP server in it and
// opens a session. The caller must Close it.
func (r *Runner) OpenChat(ctx context.Context, req ChatRequest) (*Chat, error) {
	if !HasACP(req.Harness) {
		return nil, fmt.Errorf("%s has no ACP server for a chat", req.Harness)
	}
	rr := *r
	if req.OpenCodeModel != "" {
		rr.Config.OpenCode.Model = req.OpenCodeModel
	}
	var sess session
	var err error
	if req.Source != nil {
		rr.Git = func(ctx context.Context, dir string, args ...string) (string, error) {
			return r.Git(ctx, dir, append(safeGit(), args...)...)
		}
		sess, err = rr.checkout(ctx, *req.Source)
	} else {
		sess, err = rr.openSession(ctx, Request{
			Dir: req.Dir, Harness: req.Harness, Args: nil, Print: false, InPlace: false,
		})
	}
	if err != nil {
		return nil, err
	}
	chat, err := rr.startChat(ctx, sess, req)
	if err != nil {
		rr.finish(context.WithoutCancel(ctx), sess)
		return nil, err
	}
	return chat, nil
}

func (r *Runner) startChat(
	ctx context.Context, sess session, req ChatRequest,
) (*Chat, error) {
	h, env, egress, err := r.harness(req.Harness)
	if err != nil {
		return nil, err
	}
	dir := sess.path
	if dir == "" {
		dir = req.Dir
	}
	checkout, mounts := r.workspace(ctx, dir)
	sessionDir := r.sessionDir(checkout, req.Harness)
	if err = os.MkdirAll(sessionDir, stateDirPerm); err != nil {
		return nil, fmt.Errorf("session directory: %w", err)
	}
	outcomeDir, err := os.MkdirTemp(r.TempDir, "wardcase-outcome-")
	if err != nil {
		return nil, fmt.Errorf("outcome directory: %w", err)
	}
	maps(env, h.SessionEnv(vm.SandboxSessionDir))
	if err = r.addMCP(&egress, env, true, checkout); err != nil {
		_ = os.RemoveAll(outcomeDir)
		return nil, err
	}
	env["WARDCASE_RUN"] = randomID()
	env[vm.MCPEnv] = vm.MCPOff
	proxy, err := r.NewProxy(egress)
	if err != nil {
		_ = os.RemoveAll(outcomeDir)
		return nil, fmt.Errorf("the egress proxy: %w", err)
	}
	box, err := r.Box.Open(ctx, vm.SandboxSpec{
		CloneDir: checkout, OutcomeDir: outcomeDir, SessionDir: sessionDir,
		Env: env, Command: nil, Proxy: proxy, Stdin: "", Log: nil,
		Mounts: mounts, SkipInstall: true,
	})
	if err != nil {
		_ = os.RemoveAll(outcomeDir)
		return nil, fmt.Errorf("start the VM: %w", err)
	}
	attacher, ok := box.(vm.Attacher)
	if !ok {
		box.Close()
		_ = os.RemoveAll(outcomeDir)
		return nil, errors.New("this sandbox cannot stream a process's stdio")
	}

	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	c := &Chat{
		r: r, sess: sess, box: box, client: nil, id: "", outcome: outcomeDir,
		stdin: stdinW, stop: stop, attached: make(chan error, 1),
		closeOnce: sync.Once{}, left: noLeftover(), removed: false,
	}
	go func() {
		_, attachErr := attacher.Attach(runCtx, h.ACP(), stdinR, stdoutW, false)
		_ = stdoutW.CloseWithError(io.EOF)
		_ = stdinR.Close()
		c.attached <- attachErr
	}()
	c.client = acp.NewClient(stdoutR, stdinW, c.onEvent(req.OnEvent))
	go func() { _ = c.client.Serve(runCtx) }()

	if err = c.handshake(ctx); err != nil {
		c.shutdown()
		return nil, err
	}
	return c, nil
}

func (c *Chat) onEvent(next func(acp.Event)) func(acp.Event) {
	return func(e acp.Event) {
		if e.Kind == "permission" {
			fmt.Fprintf(c.r.Notes, "wardcase: allowed a tool call (%s): %s\n",
				e.Option, e.Tool)
		}
		if next != nil {
			next(e)
		}
	}
}

func (c *Chat) handshake(ctx context.Context) error {
	err := c.client.Initialize(ctx)
	if err == nil {
		c.id, err = c.client.NewSession(ctx, vm.SandboxCloneDir)
	}
	if err == nil {
		return nil
	}
	select {
	case attachErr := <-c.attached:
		c.attached <- attachErr
		return fmt.Errorf("the ACP server stopped before the session opened: %w",
			errors.Join(err, attachErr))
	default:
		return fmt.Errorf("open the ACP session: %w", err)
	}
}

// Branch is the session's branch; Path its worktree.
func (c *Chat) Branch() string { return c.sess.branch }

// Path is the worktree the session works in.
func (c *Chat) Path() string { return c.sess.path }

// Prompt sends a prompt and returns the stop reason once the turn ends; the
// updates reach ChatRequest.OnEvent meanwhile.
func (c *Chat) Prompt(ctx context.Context, text string) (string, error) {
	return c.client.Prompt(ctx, c.id, text)
}

// Cancel asks the agent to stop the turn in progress.
func (c *Chat) Cancel() error { return c.client.Cancel(c.id) }

// Close ends the session, destroys the VM and reports what the worktree holds.
func (c *Chat) Close(ctx context.Context) (Leftover, bool) {
	c.closeOnce.Do(func() {
		c.shutdown()
		c.left, c.removed = c.r.finish(context.WithoutCancel(ctx), c.sess)
	})
	return c.left, c.removed
}

// shutdown stops the harness, then the VM and its leftovers.
func (c *Chat) shutdown() {
	_ = c.stdin.Close()
	select {
	case err := <-c.attached:
		c.attached <- err
	case <-time.After(closeGrace):
		c.stop()
		last := <-c.attached
		_ = last
	}
	c.stop()
	c.box.Close()
	_ = os.RemoveAll(c.outcome)
}

// HasACP says the harness has an ACP server to chat with.
func HasACP(harness string) bool {
	switch harness {
	case OpenCode:
		return vm.OpenCode{Model: ""}.ACP() != nil
	case Claude:
		return vm.ClaudeCode{}.ACP() != nil
	case Codex:
		return vm.Codex{}.ACP() != nil
	case Gemini:
		return vm.Gemini{}.ACP() != nil
	}
	return false
}

func noLeftover() Leftover {
	return Leftover{Uncommitted: 0, Unpushed: 0, Own: 0, PR: "", PRURL: ""}
}
