package ward

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"tools.xdoubleu.com/wardcase/vm"
)

// SessionRequest is one headless run on a branch of a repository.
type SessionRequest struct {
	Source
	// Prompt is the task.
	Prompt string
	// Harness runs first.
	Harness string
	// Fallback takes over at Harness's usage limit when Overflow is auto.
	Fallback string
	// Overflow is OverflowOff (the default) or OverflowAuto.
	Overflow string
	// OpenCodeModel is the OpenRouter model id when OpenCode runs.
	OpenCodeModel string
	// Dir runs the session in this existing checkout on its current branch
	// instead of cloning Source: no new branch, and the checkout is never removed.
	Dir string
	// ResumeID continues this session of Harness.
	ResumeID string
	// Extra adds to the session's VM; nil adds nothing.
	Extra *SessionExtra
	// Marketplaces are the skills marketplaces the session gets.
	Marketplaces []MarketplaceConfig
}

// SessionExtra is what a caller that owns the job around the session adds to it.
type SessionExtra struct {
	// Env is added to the VM's environment, over wardcase's own.
	Env map[string]string
	// OutcomeDir is mounted as the outcome directory and kept; empty means a
	// temporary one wardcase removes.
	OutcomeDir string
	// SessionDir keeps the harness's sessions instead of wardcase's own directory
	// for the checkout, so a session an earlier run left can be resumed.
	SessionDir string
	// Board and MCP are reached through the egress proxy, as in a chat.
	Board *Board
	MCP   []vm.MCPServer
}

// SessionResult is how a session ended.
type SessionResult struct {
	ExitCode int
	// Harness is the one that ran last.
	Harness string
	// Continued says Fallback took over at a usage limit.
	Continued bool
	// Limited says the last harness stopped at its usage limit.
	Limited bool
	// Branch is the session's branch; Path its worktree, empty once removed.
	Branch string
	Path   string
	// SessionID and Model are the last harness's, when its output named them.
	SessionID string
	Model     string
	// Left is what the worktree held when the run ended.
	Left Leftover
	// Marketplaces are the marketplaces the session used, at their commits.
	Marketplaces []MarketplaceUse
}

// chatRequest is the part of the extras that reaches the proxy, in a chat's terms.
func (x *SessionExtra) chatRequest() ChatRequest {
	return ChatRequest{
		Source: nil, Dir: "", Harness: "",
		OpenCodeModel: "", Board: x.Board, MCP: x.MCP, Order: nil, OnEvent: nil,
	}
}

// RunSession clones or reuses the cached clone of the repository, runs the
// harness headless in a new worktree of it and reports what it left.
func (r *Runner) RunSession(
	ctx context.Context,
	req SessionRequest,
) (SessionResult, error) {
	cfg, err := sessionConfig(req)
	if err != nil {
		return failed(), err
	}
	rr := *r
	rr.Config = cfg
	rr.extra = req.Extra
	rr.skills = &skillLog{mu: sync.Mutex{}, used: nil}
	rr.Git = func(ctx context.Context, dir string, args ...string) (string, error) {
		return r.Git(ctx, dir, append(safeGit(), args...)...)
	}
	var sess session
	if req.Dir != "" {
		sess, err = rr.existing(ctx, req.Dir, req.Branch)
	} else {
		sess, err = rr.checkout(ctx, req.Source)
	}
	if err != nil {
		return failed(), err
	}
	res, runErr := rr.loop(ctx, sess, Request{
		Dir: sess.path, Harness: req.Harness, Args: []string{req.Prompt},
		Print: true, InPlace: false,
	}, req.ResumeID)
	var left Leftover
	removed := false
	if req.Dir != "" {
		left = rr.inspect(context.WithoutCancel(ctx), sess)
	} else {
		left, removed = rr.finish(context.WithoutCancel(ctx), sess)
	}
	out := SessionResult{
		ExitCode: res.code, Harness: res.harness, Continued: res.continued,
		Limited: res.limited, Branch: sess.branch, Path: sess.path, Left: left,
		SessionID: res.sessionID, Model: res.model, Marketplaces: rr.skills.used,
	}
	if removed {
		out.Path = ""
	}
	return out, runErr
}

// existing is the caller's own checkout on its branch, which wardcase leaves in
// place. A non-empty branch must be the one checked out.
func (r *Runner) existing(ctx context.Context, dir, branch string) (session, error) {
	top, ok := r.toplevel(ctx, dir)
	if !ok {
		return none(), fmt.Errorf("%s is not a git checkout", dir)
	}
	head, err := r.Git(ctx, top, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return none(), fmt.Errorf("read the checkout's branch: %w", err)
	}
	head = strings.TrimSpace(head)
	if head == "HEAD" {
		return none(), errors.New("the checkout has no branch checked out")
	}
	if branch != "" && branch != head {
		return none(), fmt.Errorf("the checkout is on branch %s, not %s", head, branch)
	}
	return session{path: top, branch: head, home: top}, nil
}

func failed() SessionResult {
	return SessionResult{
		ExitCode: -1, Harness: "", Continued: false, Limited: false, Branch: "",
		SessionID: "", Model: "", Path: "", Marketplaces: nil,
		Left: Leftover{Uncommitted: 0, Unpushed: 0, Own: 0, PR: "", PRURL: ""},
	}
}

func sessionConfig(req SessionRequest) (Config, error) {
	if !ValidHarness(req.Harness) {
		return Config{}, fmt.Errorf("unknown harness %q", req.Harness)
	}
	if req.Prompt == "" {
		return Config{}, errors.New("a session needs a prompt")
	}
	cfg := Config{
		Order: []string{req.Harness}, Overflow: OverflowOff,
		OpenCode: OpenCodeConfig{Model: req.OpenCodeModel}, MCP: nil,
		Marketplace: req.Marketplaces,
	}
	switch req.Overflow {
	case "", OverflowOff:
	case OverflowAuto:
		cfg.Overflow = OverflowAuto
		if req.Fallback != "" {
			cfg.Order = append(cfg.Order, req.Fallback)
		}
	default:
		return Config{}, fmt.Errorf("overflow %q must be auto or off", req.Overflow)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
