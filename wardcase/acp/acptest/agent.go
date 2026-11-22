// Package acptest is a scripted ACP agent for tests: it stands in for a harness's
// ACP server on a pipe.
package acptest

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"tools.xdoubleu.com/wardcase/acp"
)

const sessionID = "s1"

// Agent answers a prompt with a message chunk, a permission request and a closing
// chunk. A prompt reading "wait" holds the turn until session/cancel.
type Agent struct {
	mu       sync.Mutex
	outcomes []json.RawMessage
	prompts  []string
	cancel   chan struct{}
	// Options are offered with each permission request; empty means the usual
	// reject_once, allow_once, allow_always.
	Options []map[string]string
}

// New returns an Agent.
func New() *Agent {
	return &Agent{
		mu: sync.Mutex{}, outcomes: nil, prompts: nil, cancel: make(chan struct{}, 1),
		Options: nil,
	}
}

// Outcomes are the answers the client gave to permission requests.
func (a *Agent) Outcomes() []json.RawMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]json.RawMessage(nil), a.outcomes...)
}

// Prompts are the prompt texts the agent received.
func (a *Agent) Prompts() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.prompts...)
}

// Serve runs the agent on r and w until r ends.
func (a *Agent) Serve(ctx context.Context, r io.Reader, w io.Writer) {
	var conn *acp.Conn
	conn = acp.NewConn(r, w, func(
		ctx context.Context, method string, params json.RawMessage,
	) (any, error) {
		switch method {
		case acp.MethodInitialize:
			return map[string]any{acp.KeyProtocolVersion: acp.ProtocolVersion}, nil
		case acp.MethodSessionNew:
			return map[string]string{acp.KeySessionID: sessionID}, nil
		case acp.MethodSessionCancel:
			select {
			case a.cancel <- struct{}{}:
			default:
			}
			return nil, nil //nolint:nilnil // a notification
		case acp.MethodSessionPrompt:
			return a.prompt(ctx, conn, params)
		}
		return nil, &acp.RPCError{Code: -32601, Message: "unsupported", Data: nil}
	})
	_ = conn.Serve(ctx)
}

func (a *Agent) update(conn *acp.Conn, text string) {
	_ = conn.Notify(acp.MethodSessionUpdate, map[string]any{
		acp.KeySessionID: sessionID,
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]string{acp.KeyType: "text", acp.KeyText: text},
		},
	})
}

func (a *Agent) prompt(
	ctx context.Context, conn *acp.Conn, params json.RawMessage,
) (any, error) {
	var p struct {
		Prompt []struct {
			Text string `json:"text"`
		} `json:"prompt"`
	}
	_ = json.Unmarshal(params, &p)
	text := ""
	if len(p.Prompt) > 0 {
		text = p.Prompt[0].Text
	}
	a.mu.Lock()
	a.prompts = append(a.prompts, text)
	a.mu.Unlock()
	if text == "wait" {
		select {
		case <-a.cancel:
		case <-ctx.Done():
		}
		return map[string]string{acp.KeyStopReason: acp.StopCancelled}, nil
	}
	a.update(conn, "working on "+text)
	options := a.Options
	if options == nil {
		options = []map[string]string{
			{"optionId": "no", "name": "Reject", "kind": "reject_once"},
			{"optionId": "yes", "name": "Allow", "kind": "allow_once"},
			{"optionId": "always", "name": "Always", "kind": "allow_always"},
		}
	}
	var out json.RawMessage
	err := conn.Call(ctx, acp.MethodRequestPermission, map[string]any{
		acp.KeySessionID: sessionID,
		"toolCall":       map[string]string{"toolCallId": "t1", "title": "bash"},
		"options":        options,
	}, &out)
	if err != nil {
		return nil, err //nolint:wrapcheck // a test double
	}
	a.mu.Lock()
	a.outcomes = append(a.outcomes, out)
	a.mu.Unlock()
	a.update(conn, "done")
	return map[string]string{acp.KeyStopReason: acp.StopEndTurn}, nil
}
