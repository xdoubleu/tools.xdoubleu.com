package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"tools.xdoubleu.com/wardcase/session"
)

// SessionAPI is wardcase's session runner, which runs one headless session.
type SessionAPI interface {
	Run(ctx context.Context, req session.Request) (session.Result, error)
}

// NewSessionAPI is the real session runner.
func NewSessionAPI(opts session.Options) SessionAPI {
	return session.New(opts)
}

// viaSession says the job's runs go through wardcase's session API: a Claude job
// on a sandbox that keeps a VM between runs.
func (l *jobLoop) viaSession() bool {
	if l.r.Sessions == nil {
		return false
	}
	if _, ok := l.r.harnessFor(l.job).(ClaudeCode); !ok {
		return false
	}
	_, ok := l.r.Sandbox.(SessionSandbox)
	return ok
}

// exec runs the harness once: in the job's VM session, or, for a Claude job, in
// wardcase on the job's clone and branch, which owns the on-limit policy.
func (l *jobLoop) exec(
	ctx context.Context, command []string, prompt string, log io.Writer,
) (int, error) {
	if !l.viaSession() {
		return l.session.Exec(ctx, command, prompt, log) //nolint:wrapcheck // as Exec
	}
	box, _ := l.r.Sandbox.(SessionSandbox)
	marketplaces, mkErr := session.DeviceMarketplaces()
	if mkErr != nil {
		l.note("The device's skill marketplaces are unreadable, so the job runs without: " +
			mkErr.Error())
	}
	api := l.r.Sessions(session.Options{
		Box: box, Secrets: l.sessionSecrets(),
		StateDir: filepath.Join(l.r.DataDir, "attempts"), TempDir: "",
		Out: log, Notes: log, GH: nil, Marketplaces: marketplaces,
	})
	req := l.sessionRequest(prompt)
	res, err := api.Run(ctx, req)
	if err != nil {
		return res.ExitCode, fmt.Errorf("run the session: %w", err)
	}
	for _, m := range res.Marketplaces {
		l.note("Skills marketplace " + m.Name + " at " + m.Commit)
	}
	if res.Continued {
		l.note("The Claude usage limit was reached; the work continued on " +
			res.Harness + " (" + req.OpenCodeModel + ")")
	}
	return res.ExitCode, nil
}

// sessionRequest is one run on the job's clone. A usage limit falls back only on
// the owner's choice, and never for a Reply, which waits for the limit to lift.
func (l *jobLoop) sessionRequest(prompt string) session.Request {
	env := make(map[string]string, len(l.launch.env))
	for k, v := range l.launch.env {
		env[k] = v
	}
	req := session.Request{
		Dir: l.dirs.clone, URL: "", Base: "", Branch: l.job.Branch, Token: "",
		Prompt: prompt, Harness: session.Claude, Overflow: session.OverflowOff,
		Fallback: "", OpenCodeModel: "", ResumeID: l.resume,
		Extra: &session.Extra{
			Env: env, OutcomeDir: l.dirs.outcome, SessionDir: l.dirs.session,
			Board: l.board(), MCP: l.launch.mcp,
		},
	}
	if l.r.OnLimit != nil && l.job.ResumeSessionID == "" {
		if policy, model := l.r.OnLimit(); policy == OnLimitFallback && model != "" {
			req.Overflow, req.Fallback = session.OverflowAuto, session.OpenCode
			req.OpenCodeModel = model
		}
	}
	return req
}

// board is the job's own board account, or nil for a job without one.
func (l *jobLoop) board() *session.Board {
	if l.job.MCPClientID == "" || l.job.MCPClientSecret == "" {
		return nil
	}
	//nolint:exhaustruct // the token cache starts empty
	source := &MCPTokenSource{
		ServerURL: l.r.ServerURL, ClientID: l.job.MCPClientID,
		ClientSecret: l.job.MCPClientSecret, HTTP: nil, Now: nil,
	}
	return &session.Board{
		Server: l.r.ServerURL, Token: source.Token, Rejected: source.Invalidate,
	}
}

// sessionSecrets serves wardcase the job's Claude token, and the OpenRouter key
// from the runner's store; the key stays on the host behind the egress proxy.
func (l *jobLoop) sessionSecrets() session.Secrets {
	return jobSecrets{
		claude: l.launch.env["CLAUDE_CODE_OAUTH_TOKEN"], openRouter: l.r.OpenRouterKey,
	}
}

type jobSecrets struct {
	claude     string
	openRouter func() (string, error)
}

func (s jobSecrets) Get(name string) (string, error) {
	switch {
	case name == "anthropic" && s.claude != "":
		return s.claude, nil
	case name == "openrouter" && s.openRouter != nil:
		return s.openRouter()
	}
	return "", fmt.Errorf("no secret %q on this runner", name)
}
