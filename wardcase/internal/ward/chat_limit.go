package ward

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/vm"
)

// StopLimited is the stop reason Prompt returns when the harness stopped at its
// usage limit; a "limited" event precedes it.
const StopLimited = "limited"

// Limit is a usage limit the chat's harness stopped at.
type Limit struct {
	// Harness is the one that stopped.
	Harness string
	// Resets is when the limit lifts, zero if unknown.
	Resets time.Time
	// Next is the harness to offer, the next in the config's order that has an ACP
	// server, or "" when none is left.
	Next string
}

// tape is the chat's conversation as the ACP stream showed it, so a switch can
// summarise any harness, not only those whose sessions wardcase can read.
type tape struct {
	mu    sync.Mutex
	turns []turn
	agent string
}

func newTape() *tape { return &tape{mu: sync.Mutex{}, turns: nil, agent: ""} }

func (t *tape) flush() {
	if strings.TrimSpace(t.agent) != "" {
		t.turns = append(t.turns, turn{who: whoAgent, text: strings.TrimSpace(t.agent)})
	}
	t.agent = ""
}

func (t *tape) user(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flush()
	t.turns = append(t.turns, turn{who: whoUser, text: text})
}

func (t *tape) agentText(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agent += text
}

// pending is what the agent said since the last prompt.
func (t *tape) pending() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.agent
}

func (t *tape) conversation() conversation {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flush()
	return conversation{turns: append([]turn(nil), t.turns...), todos: nil}
}

// Prompt sends a prompt and returns the stop reason once the turn ends; the
// updates reach ChatRequest.OnEvent meanwhile. At a usage limit it emits a
// "limited" event and returns StopLimited; Limit then says what to offer.
func (c *Chat) Prompt(ctx context.Context, text string) (string, error) {
	c.tape.user(text)
	c.mu.Lock()
	send, seeded := text, c.seed
	if seeded != "" {
		send = seeded + "\n\n" + text
	}
	c.mu.Unlock()

	reason, err := c.client.Prompt(ctx, c.id, send)
	if hit, resets := c.limitedBy(reason, err); hit {
		c.mu.Lock()
		c.seed = ""
		c.limit = &Limit{
			Harness: c.harness, Resets: resets, Next: c.nextHarness(),
		}
		c.mu.Unlock()
		c.emit(acp.Event{
			Kind: "limited", Update: nil, Tool: nil, Option: "", Resets: resets,
			Harness: c.nextHarness(),
		})
		return StopLimited, nil
	}
	if err == nil {
		c.mu.Lock()
		c.seed = ""
		c.mu.Unlock()
	}
	return reason, err
}

// limitedBy reads a finished turn for a usage limit: the error the agent answered
// the prompt with, or, when it stopped for another reason than finishing or being
// cancelled, what it said.
func (c *Chat) limitedBy(reason string, err error) (bool, time.Time) {
	var rpc *acp.RPCError
	switch {
	case errors.As(err, &rpc):
		return vm.LimitInError(rpc.Message + " " + string(rpc.Data))
	case err != nil:
		return false, time.Time{}
	case reason == acp.StopEndTurn || reason == acp.StopCancelled:
		return false, time.Time{}
	}
	return vm.LimitInError(c.tape.pending())
}

func (c *Chat) emit(e acp.Event) {
	if c.req.OnEvent != nil {
		c.req.OnEvent(e)
	}
}

// nextHarness is the harness after the current one in the config's order that
// has an ACP server.
func (c *Chat) nextHarness() string {
	name := c.harness
	for {
		name = c.r.Config.next(name)
		if name == "" || HasACP(name) {
			return name
		}
	}
}

// Limit is the usage limit the last prompt stopped at, if it did.
func (c *Chat) Limit() (Limit, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.limit == nil {
		return Limit{Harness: "", Resets: time.Time{}, Next: ""}, false
	}
	return *c.limit, true
}

// Switch moves the chat to another harness (and OpenCode model, when model is
// set) in the same worktree. The new session's first prompt carries a summary of
// the conversation in the handoff format, and a "switch" event marks the move.
func (c *Chat) Switch(ctx context.Context, harness, model string) error {
	if !HasACP(harness) {
		return fmt.Errorf("%s has no ACP server for a chat", harness)
	}
	text := handoff(
		Header{Version: HandoffVersion, From: c.harness, To: harness, Base: ""},
		gitState(ctx, c.r.Git, c.dir, ""), c.tape.conversation())

	c.shutdown()
	rr := *c.r
	if model != "" {
		rr.Config.OpenCode.Model = model
	}
	req := c.req
	req.Harness, req.OpenCodeModel = harness, model
	n, err := rr.startChat(ctx, c.sess, req, c.tape)
	if err != nil {
		return fmt.Errorf("continue on %s: %w", harness, err)
	}
	c.r, c.box, c.client, c.id, c.outcome = n.r, n.box, n.client, n.id, n.outcome
	c.stdin, c.stop, c.attached = n.stdin, n.stop, n.attached
	c.harness, c.req = harness, req
	c.mu.Lock()
	c.limit, c.seed = nil, text
	c.mu.Unlock()
	c.emit(acp.Event{
		Kind: "switch", Update: nil, Tool: nil, Option: "", Resets: time.Time{},
		Harness: harness,
	})
	return nil
}
