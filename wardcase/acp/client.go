package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

// ProtocolVersion is the ACP major version wardcase speaks.
const ProtocolVersion = 1

// Option kinds of a permission request.
const (
	allowOnce   = "allow_once"
	allowAlways = "allow_always"
)

// Event is something the agent reported while a prompt ran.
type Event struct {
	// Kind is "update" for a session/update, "permission" for a permission request
	// the client answered.
	Kind string
	// Update is the session/update's `update` object, as the agent sent it.
	Update json.RawMessage
	// Tool is the tool call a permission request was about, as the agent sent it,
	// and Option the option that was chosen ("" when none allowed it).
	Tool   json.RawMessage
	Option string
}

// UpdateKind is the update's `sessionUpdate` tag, such as "agent_message_chunk".
func (e Event) UpdateKind() string {
	var u struct {
		Kind string `json:"sessionUpdate"`
	}
	_ = json.Unmarshal(e.Update, &u)
	return u.Kind
}

// Text is the text of a message chunk update, or "".
func (e Event) Text() string {
	var u struct {
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(e.Update, &u)
	return u.Content.Text
}

// Client drives one agent over a stream, answering every permission request with
// allow: the sandbox is the boundary → adr-0060.
type Client struct {
	conn    *Conn
	onEvent func(Event)
}

// NewClient talks to an agent whose stdout is r and stdin is w. Call Serve, then
// Initialize.
func NewClient(r io.Reader, w io.Writer, onEvent func(Event)) *Client {
	c := &Client{conn: nil, onEvent: onEvent}
	c.conn = NewConn(r, w, c.handle)
	return c
}

// Serve reads the agent's messages until it hangs up.
func (c *Client) Serve(ctx context.Context) error { return c.conn.Serve(ctx) }

// Done is closed once the agent's stream has ended.
func (c *Client) Done() <-chan struct{} { return c.conn.Done() }

func (c *Client) emit(e Event) {
	if c.onEvent != nil {
		c.onEvent(e)
	}
}

func (c *Client) handle(
	_ context.Context, method string, params json.RawMessage,
) (any, error) {
	switch method {
	case MethodSessionUpdate:
		var p struct {
			Update json.RawMessage `json:"update"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err //nolint:wrapcheck // a JSON-RPC error to the agent
		}
		c.emit(Event{Kind: "update", Update: p.Update, Tool: nil, Option: ""})
		return nil, nil //nolint:nilnil // a notification has no result
	case MethodRequestPermission:
		return c.permission(params), nil
	}
	return nil, unsupported(method)
}

type permissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

// permission allows once when it can, else always; a request with no allowing
// option is cancelled.
func (c *Client) permission(params json.RawMessage) map[string]permissionOutcome {
	var p struct {
		ToolCall json.RawMessage `json:"toolCall"`
		Options  []struct {
			ID   string `json:"optionId"`
			Kind string `json:"kind"`
		} `json:"options"`
	}
	_ = json.Unmarshal(params, &p)
	chosen := ""
	for _, kind := range []string{allowOnce, allowAlways} {
		for _, o := range p.Options {
			if chosen == "" && o.Kind == kind {
				chosen = o.ID
			}
		}
	}
	c.emit(Event{Kind: "permission", Update: nil, Tool: p.ToolCall, Option: chosen})
	if chosen == "" {
		cancelled := permissionOutcome{Outcome: StopCancelled, OptionID: ""}
		return map[string]permissionOutcome{outcomeKey: cancelled}
	}
	selected := permissionOutcome{Outcome: "selected", OptionID: chosen}
	return map[string]permissionOutcome{outcomeKey: selected}
}

// Initialize negotiates the protocol. The client offers no file or terminal
// access: the agent uses its own tools inside the VM.
func (c *Client) Initialize(ctx context.Context) error {
	var res struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	err := c.conn.Call(ctx, MethodInitialize, map[string]any{
		KeyProtocolVersion:   ProtocolVersion,
		"clientCapabilities": map[string]any{},
		"clientInfo":         map[string]string{"name": "wardcase"},
	}, &res)
	if err != nil {
		return err
	}
	if res.ProtocolVersion != ProtocolVersion {
		return errors.New("the agent speaks a different ACP version")
	}
	return nil
}

// NewSession opens a session working in cwd, which is a path inside the agent's
// VM, and returns its id.
func (c *Client) NewSession(ctx context.Context, cwd string) (string, error) {
	var res struct {
		SessionID string `json:"sessionId"`
	}
	err := c.conn.Call(ctx, MethodSessionNew, map[string]any{
		"cwd": cwd, "mcpServers": []any{},
	}, &res)
	if err != nil {
		return "", err
	}
	if res.SessionID == "" {
		return "", errors.New("the agent returned no session id")
	}
	return res.SessionID, nil
}

// Prompt sends text and returns the stop reason once the agent's turn ends; its
// updates arrive as events meanwhile.
func (c *Client) Prompt(ctx context.Context, sessionID, text string) (string, error) {
	block := map[string]string{KeyType: "text", KeyText: text}
	return c.PromptBlocks(ctx, sessionID, []map[string]string{block})
}

// PromptBlocks is Prompt with the content blocks given as they go on the wire.
func (c *Client) PromptBlocks(
	ctx context.Context, sessionID string, blocks any,
) (string, error) {
	var res struct {
		StopReason string `json:"stopReason"`
	}
	err := c.conn.Call(ctx, MethodSessionPrompt, map[string]any{
		KeySessionID: sessionID, "prompt": blocks,
	}, &res)
	return res.StopReason, err
}

// Cancel asks the agent to stop the turn in progress; the prompt then returns
// with the stop reason "cancelled".
func (c *Client) Cancel(sessionID string) error {
	return c.conn.Notify(MethodSessionCancel,
		map[string]string{KeySessionID: sessionID})
}
