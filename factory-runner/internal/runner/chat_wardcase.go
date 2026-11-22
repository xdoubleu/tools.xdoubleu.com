package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/session"
)

// errNoSession is a sandbox that cannot keep a VM between commands, which a chat
// needs.
var errNoSession = errors.New("this sandbox cannot hold a chat's VM")

// openWardcaseChat opens the chat in wardcase: a worktree of the repo on the
// chat's branch and the harness's ACP server in a VM, with the board as the chat's
// own account and the device's MCP servers behind the egress proxy.
func (r *JobRunner) openWardcaseChat(
	ctx context.Context,
	spec ChatSpec,
) (ChatSession, error) {
	box, ok := r.Sandbox.(SessionSandbox)
	if !ok {
		return nil, errNoSession
	}
	state := filepath.Join(r.DataDir, "chats")
	runner := session.New(session.Options{
		Box: box, Secrets: openRouterSecret{r.OpenRouterKey}, StateDir: state,
		TempDir: "", Out: nil, Notes: nil, GH: nil,
	})
	job := spec.Job
	remote := GitHubURL
	if r.Git != nil && r.Git.RemoteURL != nil {
		remote = r.Git.RemoteURL
	}
	req := session.ChatRequest{
		URL:  remote(job.RepoOwner, job.RepoName),
		Base: job.DefaultBranch, Branch: job.Branch, Token: spec.Token,
		Harness: session.OpenCode, OpenCodeModel: spec.Model,
		Board: nil, MCP: spec.MCP,
		OnEvent: func(e acp.Event) { forwardEvent(spec.OnEvent, e) },
	}
	if job.MCPClientID != "" && job.MCPClientSecret != "" {
		//nolint:exhaustruct // the token cache starts empty
		source := &MCPTokenSource{
			ServerURL: r.ServerURL, ClientID: job.MCPClientID,
			ClientSecret: job.MCPClientSecret, HTTP: nil, Now: nil,
		}
		req.Board = &session.Board{
			Server: r.ServerURL, Token: source.Token, Rejected: source.Invalidate,
		}
	}
	chat, err := runner.Start(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start the session: %w", err)
	}
	return wardcaseChat{chat}, nil
}

// forwardEvent turns an ACP event into the transcript's kind and payload.
func forwardEvent(next func(kind, payload string), e acp.Event) {
	switch e.Kind {
	case "update":
		next(eventUpdate, string(e.Update))
	case "permission":
		next(eventPermission,
			fmt.Sprintf(`{"tool":%s,"option":%q}`, orNull(e.Tool), e.Option))
	}
}

func orNull(raw []byte) string {
	if len(raw) == 0 {
		return "null"
	}
	return string(raw)
}

// wardcaseChat adapts wardcase's Chat to ChatSession.
type wardcaseChat struct{ c *session.Chat }

func (w wardcaseChat) Prompt(ctx context.Context, text string) (string, error) {
	return w.c.Prompt(ctx, text) //nolint:wrapcheck // names the method
}

func (w wardcaseChat) Cancel() error {
	return w.c.Cancel() //nolint:wrapcheck // names the method
}

func (w wardcaseChat) Close(ctx context.Context) ChatLeft {
	res := w.c.Close(ctx)
	return ChatLeft{
		Branch: res.Branch, Path: res.Path,
		Uncommitted: res.Uncommitted, Unpushed: res.Unpushed,
	}
}

// openRouterSecret serves wardcase the one secret an OpenCode chat uses, from the
// runner's own store.
type openRouterSecret struct{ key func() (string, error) }

func (s openRouterSecret) Get(name string) (string, error) {
	if name != "openrouter" || s.key == nil {
		return "", fmt.Errorf("no secret %q on this runner", name)
	}
	return s.key()
}
