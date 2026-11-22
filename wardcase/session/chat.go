package session

import (
	"context"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/internal/ward"
)

// Event is something the harness reported during a turn: a session/update, or a
// permission request wardcase allowed.
type Event = acp.Event

// ChatRequest is one interactive session: the harness's ACP server runs in the
// VM, on a new branch in its own worktree. OpenCode, Codex and Gemini have one.
type ChatRequest struct {
	// URL, Base, Branch and Token are as in Request.
	URL, Base, Branch, Token string
	// Harness is OpenCode, Codex or Gemini.
	Harness string
	// OpenCodeModel is the OpenRouter model id.
	OpenCodeModel string
	// OnEvent receives every event, one at a time, in order.
	OnEvent func(Event)
}

// Chat is a running interactive session. Every permission request the harness
// makes is allowed: the VM is the boundary.
type Chat struct {
	c       *ward.Chat
	harness string
}

// Start opens a Chat. The caller must Close it.
func (s *Runner) Start(ctx context.Context, req ChatRequest) (*Chat, error) {
	c, err := s.r.OpenChat(ctx, ward.ChatRequest{
		Source: &ward.Source{
			URL: req.URL, Base: req.Base, Branch: req.Branch, Token: req.Token,
		},
		Dir: "", Harness: req.Harness, OpenCodeModel: req.OpenCodeModel,
		OnEvent: req.OnEvent,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // ward's errors name their step
	}
	return &Chat{c: c, harness: req.Harness}, nil
}

// Prompt sends text and returns the turn's stop reason ("end_turn", "cancelled",
// …) once it ends; the events arrive meanwhile.
func (c *Chat) Prompt(ctx context.Context, text string) (string, error) {
	return c.c.Prompt(ctx, text) //nolint:wrapcheck // acp's errors name the method
}

// Cancel asks the harness to stop the turn in progress; Prompt then returns the
// stop reason "cancelled".
func (c *Chat) Cancel() error {
	return c.c.Cancel() //nolint:wrapcheck // acp's errors name the method
}

// Close ends the session, destroys the VM and returns the leftover report; a
// second Close returns the same one. Result carries only the worktree's state.
func (c *Chat) Close(ctx context.Context) Result {
	left, removed := c.c.Close(ctx)
	res := Result{
		ExitCode: 0, Harness: c.harness, Continued: false, Limited: false,
		Branch: c.c.Branch(), Path: c.c.Path(),
		Uncommitted: left.Uncommitted, Unpushed: left.Unpushed,
	}
	if removed {
		res.Path = ""
	}
	return res
}
