// Package acp is a small Agent Client Protocol (JSON-RPC 2.0 over newline-delimited
// stdio) client and agent, covering the methods wardcase needs → adr-0060.
package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	maxLine = 16 << 20
	bufSize = 64 << 10

	codeMethodNotFound = -32601
	codeInternal       = -32603
)

// Handler answers an incoming request or notification. For a notification the
// result is dropped. An error becomes a JSON-RPC error response.
type Handler func(
	ctx context.Context, method string, params json.RawMessage,
) (any, error)

// RPCError is a JSON-RPC error a peer answered with.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("acp error %d: %s", e.Code, e.Message)
}

// ErrClosed is returned by calls on a connection whose peer hung up.
var ErrClosed = errors.New("acp connection closed")

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

type reply struct {
	result json.RawMessage
	err    error
}

// Conn is one end of a JSON-RPC connection. Notifications are handled inline, in
// arrival order; requests run on their own goroutine so a long one (a prompt)
// does not block a cancel behind it.
type Conn struct {
	r       *bufio.Reader
	handler Handler

	wmu sync.Mutex
	w   io.Writer

	mu      sync.Mutex
	nextID  int
	pending map[int]chan reply
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewConn serves handler on r and w; call Serve to start reading.
func NewConn(r io.Reader, w io.Writer, handler Handler) *Conn {
	return &Conn{
		r: bufio.NewReaderSize(r, bufSize), handler: handler, wmu: sync.Mutex{}, w: w,
		mu: sync.Mutex{}, nextID: 0, pending: map[int]chan reply{},
		done: make(chan struct{}), wg: sync.WaitGroup{},
	}
}

// Done is closed once Serve has returned.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Serve reads until the peer closes the stream or ctx ends, then fails the calls
// still waiting. Lines that are not JSON objects are skipped: the harness's own
// stderr shares the stream.
func (c *Conn) Serve(ctx context.Context) error {
	defer close(c.done)
	defer c.failPending()
	defer c.wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		line, err := readLine(c.r)
		if len(bytes.TrimSpace(line)) > 0 {
			c.dispatch(ctx, bytes.TrimSpace(line))
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read acp: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		line = append(line, part...)
		if errors.Is(err, bufio.ErrBufferFull) {
			if len(line) > maxLine {
				return nil, errors.New("acp line too long")
			}
			continue
		}
		return line, err
	}
}

func (c *Conn) failPending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		ch <- reply{result: nil, err: ErrClosed}
		delete(c.pending, id)
	}
}

func (c *Conn) dispatch(ctx context.Context, line []byte) {
	if line[0] != '{' {
		return
	}
	var m message
	if json.Unmarshal(line, &m) != nil {
		return
	}
	switch {
	case m.Method == "":
		c.deliver(m)
	case m.ID == nil:
		if c.handler != nil {
			_, _ = c.handler(ctx, m.Method, m.Params)
		}
	default:
		id := *m.ID
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.answer(ctx, id, m)
		}()
	}
}

func (c *Conn) answer(ctx context.Context, id json.RawMessage, m message) {
	if c.handler == nil {
		var out message
		out.JSONRPC, out.ID, out.Error = jsonrpcVersion, &id, unsupported(m.Method)
		_ = c.send(out)
		return
	}
	result, err := c.handler(ctx, m.Method, m.Params)
	var out message
	out.JSONRPC, out.ID = jsonrpcVersion, &id
	var rpc *RPCError
	switch {
	case errors.As(err, &rpc):
		out.Error = rpc
	case err != nil:
		out.Error = &RPCError{Code: codeInternal, Message: err.Error()}
	default:
		raw, mErr := json.Marshal(result)
		if mErr != nil {
			out.Error = &RPCError{Code: codeInternal, Message: mErr.Error()}
		} else {
			out.Result = raw
		}
	}
	_ = c.send(out)
}

func (c *Conn) deliver(m message) {
	if m.ID == nil {
		return
	}
	var id int
	if json.Unmarshal(*m.ID, &id) != nil {
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if !ok {
		return
	}
	if m.Error != nil {
		ch <- reply{result: nil, err: m.Error}
		return
	}
	ch <- reply{result: m.Result, err: nil}
}

func (c *Conn) send(m message) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode acp message: %w", err)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err = c.w.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write acp: %w", err)
	}
	return nil
}

// Call sends a request and decodes the answer into result (nil to discard it).
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	ch := make(chan reply, 1)
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return ErrClosed
	default:
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()

	rawID := json.RawMessage(fmt.Sprint(id))
	var req message
	req.JSONRPC, req.ID, req.Method, req.Params = jsonrpcVersion, &rawID, method, raw
	if err = c.send(req); err != nil {
		c.forget(id)
		return err
	}
	select {
	case rep := <-ch:
		if rep.err != nil {
			return rep.err
		}
		if result == nil {
			return nil
		}
		if err = json.Unmarshal(rep.result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
		return nil
	case <-ctx.Done():
		c.forget(id)
		return fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

func (c *Conn) forget(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// Notify sends a notification.
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	var note message
	note.JSONRPC, note.Method, note.Params = jsonrpcVersion, method, raw
	return c.send(note)
}
