// Package session runs one headless agent session from a repository URL and a
// branch: wardcase clones or reuses its cached clone, cuts a worktree for the new
// branch, runs the harness in the VM and reports what it left. A caller needs no
// checkout of its own.
package session

import (
	"context"
	"io"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

// Harness names, and the overflow policies, as Request takes them.
const (
	Claude       = ward.Claude
	OpenCode     = ward.OpenCode
	Codex        = ward.Codex
	Gemini       = ward.Gemini
	OverflowOff  = ward.OverflowOff
	OverflowAuto = ward.OverflowAuto
)

// Secrets holds the model credentials, which stay on the host: Claude Code's
// token is the only one that reaches the VM, and only through the egress proxy.
type Secrets interface {
	// Get returns the named secret ("anthropic", "openrouter", "openai", "gemini").
	Get(name string) (string, error)
}

// Options is what a caller supplies once, for every session.
type Options struct {
	// Box starts the VM.
	Box vm.SessionSandbox
	// Secrets supplies the harnesses' credentials.
	Secrets Secrets
	// StateDir keeps the cached clones, worktrees and harness sessions.
	StateDir string
	// TempDir holds each run's outcome directory; empty means os.TempDir().
	TempDir string
	// Out receives the harness's output; nil discards it.
	Out io.Writer
	// Notes receives wardcase's own messages; nil discards them.
	Notes io.Writer
	// GH runs gh in a directory, to report the branch's pull request; nil leaves
	// pull requests unreported.
	GH func(ctx context.Context, dir string, args ...string) (string, error)
}

// Request is one session.
type Request struct {
	// Dir runs the session in this existing checkout on its current branch, which
	// Branch must name when set: nothing is cloned or cut, and the checkout is
	// never removed. URL, Base and Token are unused. The VM mounts it as it is;
	// wardcase's own git on it runs with hooks off.
	Dir string
	// URL is the repository, cloned over HTTPS or from a local path.
	URL string
	// Base is the branch the new one is cut from.
	Base string
	// Branch is the new branch the session commits on.
	Branch string
	// Token authenticates the clone; empty uses the host git's own credentials,
	// such as the `gh` login. It never enters the VM.
	Token string
	// Prompt is the task.
	Prompt string
	// Harness is Claude, OpenCode, Codex or Gemini.
	Harness string
	// Overflow is OverflowOff (the default) or OverflowAuto, which continues on
	// Fallback in the same worktree when Harness hits its usage limit.
	Overflow string
	Fallback string
	// OpenCodeModel is the OpenRouter model id when OpenCode runs.
	OpenCodeModel string
	// ResumeID continues this session of Harness (a Result.SessionID of an earlier
	// run, kept in StateDir) instead of starting a new one.
	ResumeID string
}

// Result is how a session ended.
type Result struct {
	// ExitCode is the last harness's.
	ExitCode int
	// Harness is the one that ran last; Continued says Fallback took over.
	Harness   string
	Continued bool
	// Limited says the last harness stopped at its usage limit.
	Limited bool
	// SessionID and Model are the last harness's, when its output named them
	// (Claude Code; other harnesses report the session id only if they print one).
	SessionID string
	Model     string
	// Branch is the session's branch; Path is its worktree, empty once wardcase
	// removed it because nothing was left. With Request.Dir, Path is that checkout.
	Branch string
	Path   string
	// Uncommitted counts changed files, Unpushed the commits on no remote.
	Uncommitted int
	Unpushed    int
}

// Left says the worktree holds work not yet shipped.
func (r Result) Left() bool { return r.Uncommitted > 0 || r.Unpushed > 0 }

// Runner runs sessions.
type Runner struct {
	r *ward.Runner
}

// New returns a Runner on the host's git; the VM is Options.Box.
func New(opts Options) *Runner {
	out, notes := opts.Out, opts.Notes
	if out == nil {
		out = io.Discard
	}
	if notes == nil {
		notes = io.Discard
	}
	return &Runner{r: &ward.Runner{
		Box: opts.Box, Secrets: readOnly{opts.Secrets}, Config: emptyConfig(),
		StateDir: opts.StateDir, TempDir: opts.TempDir,
		Git: ward.HostGit, GitEnv: ward.HostGitEnv, GH: opts.GH,
		NewProxy: func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
			return vm.NewEgressProxy(cfg)
		},
		In: nil, Out: out, Notes: notes, Terminal: false, Ask: nil, Now: nil,
	}}
}

// Run runs the session, headless, and returns how it ended. An error means it
// could not start or run; a harness that ran and failed is ExitCode.
func (s *Runner) Run(ctx context.Context, req Request) (Result, error) {
	res, err := s.r.RunSession(ctx, ward.SessionRequest{
		Source: ward.Source{
			URL: req.URL, Base: req.Base, Branch: req.Branch, Token: req.Token,
		},
		Prompt: req.Prompt, Harness: req.Harness, Fallback: req.Fallback,
		Overflow: req.Overflow, OpenCodeModel: req.OpenCodeModel,
		Dir: req.Dir, ResumeID: req.ResumeID,
	})
	return Result{
		ExitCode: res.ExitCode, Harness: res.Harness, Continued: res.Continued,
		Limited: res.Limited, Branch: res.Branch, Path: res.Path,
		SessionID: res.SessionID, Model: res.Model,
		Uncommitted: res.Left.Uncommitted, Unpushed: res.Left.Unpushed,
	}, err
}

// readOnly makes a Get-only Secrets a ward.Secrets; a session never stores one.
type readOnly struct{ Secrets }

func (readOnly) Set(string, string) error { return ward.ErrNoSecret }

func (readOnly) Delete(string) error { return ward.ErrNoSecret }

// emptyConfig is the device config a session does not use: each session names its
// own harnesses and overflow.
func emptyConfig() ward.Config {
	return ward.Config{
		Order: nil, Overflow: "", OpenCode: ward.OpenCodeConfig{Model: ""}, MCP: nil,
	}
}
