package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

// Backend is the session a Server fronts.
type Backend interface {
	// Open starts the session; cwd is the working directory the ADE named, which a
	// backend may ignore. Updates go to emit as they happen.
	Open(ctx context.Context, cwd string, emit func(update json.RawMessage)) error
	// Prompt runs one turn and returns its stop reason.
	Prompt(ctx context.Context, text string) (string, error)
	// Cancel stops the turn in progress.
	Cancel()
}

// Server is an ACP agent on a stream, such as an ADE's pipe to a subprocess. It
// serves one session.
type Server struct {
	// Name and Version go in the agentInfo of initialize.
	Name, Version string
	Backend       Backend

	conn *Conn
	mu   sync.Mutex
	id   string
}

// NewServer serves b on r and w; call Serve.
func NewServer(r io.Reader, w io.Writer, name, version string, b Backend) *Server {
	s := &Server{
		Name: name, Version: version, Backend: b, conn: nil, mu: sync.Mutex{}, id: "",
	}
	s.conn = NewConn(r, w, s.handle)
	return s
}

// Serve answers the ADE until it closes the stream.
func (s *Server) Serve(ctx context.Context) error { return s.conn.Serve(ctx) }

// SessionID is the id handed out by session/new, or "".
func (s *Server) sessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

func (s *Server) handle(
	ctx context.Context, method string, params json.RawMessage,
) (any, error) {
	switch method {
	case MethodInitialize:
		return map[string]any{
			KeyProtocolVersion:  ProtocolVersion,
			"agentCapabilities": map[string]any{},
			"agentInfo":         map[string]string{"name": s.Name, "version": s.Version},
		}, nil
	case MethodSessionNew:
		return s.newSession(ctx, params)
	case MethodSessionPrompt:
		return s.prompt(ctx, params)
	case MethodSessionCancel:
		s.Backend.Cancel()
		return nil, nil //nolint:nilnil // a notification has no result
	}
	return nil, unsupported(method)
}

func (s *Server) newSession(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(params, &p)
	s.mu.Lock()
	if s.id != "" {
		s.mu.Unlock()
		return nil, errors.New("wardcase serves one session per process")
	}
	s.id = "wardcase-" + randomSuffix()
	id := s.id
	s.mu.Unlock()
	err := s.Backend.Open(ctx, p.Cwd, func(update json.RawMessage) {
		_ = s.conn.Notify(MethodSessionUpdate, map[string]any{
			KeySessionID: id, "update": update,
		})
	})
	if err != nil {
		s.mu.Lock()
		s.id = ""
		s.mu.Unlock()
		return nil, err
	}
	return map[string]string{KeySessionID: id}, nil
}

func (s *Server) prompt(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			URI      string `json:"uri"`
			Resource struct {
				Text string `json:"text"`
				URI  string `json:"uri"`
			} `json:"resource"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: codeInvalidParams, Message: err.Error(), Data: nil}
	}
	if id := s.sessionID(); id == "" || id != p.SessionID {
		return nil, &RPCError{Code: codeInvalidParams, Message: "unknown session", Data: nil}
	}
	var parts []string
	for _, b := range p.Prompt {
		switch {
		case b.Text != "":
			parts = append(parts, b.Text)
		case b.Resource.Text != "":
			parts = append(parts, b.Resource.URI+":\n"+b.Resource.Text)
		case b.URI != "":
			parts = append(parts, b.URI)
		}
	}
	reason, err := s.Backend.Prompt(ctx, strings.Join(parts, "\n\n"))
	if err != nil {
		return nil, err
	}
	return map[string]string{KeyStopReason: reason}, nil
}
