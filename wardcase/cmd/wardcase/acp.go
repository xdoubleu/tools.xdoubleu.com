package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/internal/ward"
)

const acpUsage = "usage: wardcase acp [opencode|codex|gemini]"

// acpCommand runs `wardcase acp`: an ACP agent on stdio whose session runs in the
// VM, for an ADE that adds wardcase as an agent.
func acpCommand(
	ctx context.Context,
	h host,
	home string,
	args []string,
	in *os.File,
	out, errOut io.Writer,
) int {
	harness := ward.OpenCode
	if len(args) > 1 {
		fmt.Fprintln(errOut, acpUsage)
		return exitUsage
	}
	for _, name := range args {
		if !ward.HasACP(name) {
			fmt.Fprintln(errOut, acpUsage)
			return exitUsage
		}
		harness = name
	}
	cfg, err := ward.LoadConfig(ward.ConfigPath(home))
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	r, err := newRunner(ctx, h, home, cfg, in, io.Discard, errOut)
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	if err = serveACP(ctx, r, harness, in, out); err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	return 0
}

// serveACP answers the ADE on in and out until it hangs up or ctx ends, then
// closes the session, which reports what it left on the Runner's Notes.
func serveACP(
	ctx context.Context, r *ward.Runner, harness string, in io.Reader, out io.Writer,
) error {
	b := &chatBackend{r: r, harness: harness, mu: sync.Mutex{}, chat: nil}
	go func() {
		<-ctx.Done()
		b.close(ctx)
	}()
	err := acp.NewServer(in, out, "wardcase", Version, b).Serve(ctx)
	b.close(ctx)
	if err != nil {
		return fmt.Errorf("acp: %w", err)
	}
	return nil
}

// chatBackend is the acp.Server's session: a ward.Chat opened by session/new.
type chatBackend struct {
	r       *ward.Runner
	harness string

	mu   sync.Mutex
	chat *ward.Chat
}

func (b *chatBackend) Open(
	ctx context.Context, cwd string, emit func(json.RawMessage),
) error {
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return fmt.Errorf("working directory: %w", err)
		}
	}
	chat, err := b.r.OpenChat(ctx, ward.ChatRequest{
		Source: nil, Dir: cwd, Harness: b.harness, OpenCodeModel: "",
		Board: nil, MCP: nil, Order: nil,
		OnEvent: func(e acp.Event) {
			if e.Kind == "update" {
				emit(e.Update)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("start the session: %w", err)
	}
	b.mu.Lock()
	b.chat = chat
	b.mu.Unlock()
	return nil
}

func (b *chatBackend) current() *ward.Chat {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.chat
}

func (b *chatBackend) Prompt(ctx context.Context, text string) (string, error) {
	chat := b.current()
	if chat == nil {
		return "", fmt.Errorf("no session")
	}
	reason, err := chat.Prompt(ctx, text)
	if err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}
	return reason, nil
}

func (b *chatBackend) Cancel() {
	if chat := b.current(); chat != nil {
		_ = chat.Cancel()
	}
}

func (b *chatBackend) close(ctx context.Context) {
	if chat := b.current(); chat != nil {
		chat.Close(ctx)
	}
}
