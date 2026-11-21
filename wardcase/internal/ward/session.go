package ward

import (
	"context"
	"errors"
	"fmt"
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
	// Left is what the worktree held when the run ended.
	Left Leftover
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
	rr.Git = func(ctx context.Context, dir string, args ...string) (string, error) {
		return r.Git(ctx, dir, append(safeGit(), args...)...)
	}
	sess, err := rr.checkout(ctx, req.Source)
	if err != nil {
		return failed(), err
	}
	res, runErr := rr.loop(ctx, sess, Request{
		Dir: sess.path, Harness: req.Harness, Args: []string{req.Prompt},
		Print: true, InPlace: false,
	})
	left, removed := rr.finish(context.WithoutCancel(ctx), sess)
	out := SessionResult{
		ExitCode: res.code, Harness: res.harness, Continued: res.continued,
		Limited: res.limited, Branch: sess.branch, Path: sess.path, Left: left,
	}
	if removed {
		out.Path = ""
	}
	return out, runErr
}

func failed() SessionResult {
	return SessionResult{
		ExitCode: -1, Harness: "", Continued: false, Limited: false, Branch: "",
		Path: "", Left: Leftover{Uncommitted: 0, Unpushed: 0, Own: 0, PR: "", PRURL: ""},
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
		OpenCode: OpenCodeConfig{Model: req.OpenCodeModel},
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
