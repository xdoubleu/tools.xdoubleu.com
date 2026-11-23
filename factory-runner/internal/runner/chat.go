package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	// chatFlushEvery is how often the harness's events reach the server mid-turn.
	chatFlushEvery = 200 * time.Millisecond
	// chatCancelWait is how long a stopped chat's turn gets to end after Cancel
	// before the VM is destroyed under it.
	chatCancelWait = 10 * time.Second
	// chatGiveUp is how long the chat survives with no socket to the server.
	chatGiveUp = 10 * time.Minute
	chatQueue  = 64
	closeWait  = 2 * time.Minute

	eventUpdate     = "update"
	eventPermission = "permission"
	eventError      = "error"
	stateIdle       = "idle"
)

// chatInstructions is the chat's system prompt, sent ahead of the first message:
// ACP has no system prompt of its own, and OpenCode's own question tool may not
// reach the person.
const chatInstructions = "You are in a chat with a person through the factory. " +
	"When you need them to choose between options or to confirm something, call " +
	"the factory_chat_ask tool (a question, 2 to 4 options, multi_select when " +
	"several may be chosen) instead of asking in prose or with another question " +
	"tool: it shows them a card and returns their answer. When it returns status " +
	"waiting, call it again with the same ask_id until it returns the answer."

// ChatSession is the harness session of one chat, wardcase's session.Chat in
// production.
type ChatSession interface {
	// Prompt runs one turn to its end and returns why it stopped.
	Prompt(ctx context.Context, text string) (string, error)
	// Cancel asks the turn in progress to stop.
	Cancel() error
	// Close ends the session and its VM, and says what the worktree was left with.
	Close(ctx context.Context) ChatLeft
}

// ChatLeft is what a closed session left in its worktree.
type ChatLeft struct {
	Branch string
	// Path is the worktree, empty once nothing was left and it was removed.
	Path        string
	Uncommitted int
	Unpushed    int
}

// ChatSpec is what a chat's session is opened with.
type ChatSpec struct {
	Job *ClaimedJob
	// Token clones the repo, for the host's git only; empty uses the host's own
	// credentials.
	Token string
	// Model is the OpenRouter model of the OpenCode harness.
	Model string
	// MCP are the device's MCP servers the session gets beside the board.
	MCP []MCPServer
	// Instructions go to the harness ahead of the first message.
	Instructions string
	// OnEvent receives what the harness reports, one at a time, in order: kind is
	// "update" or "permission", payload JSON.
	OnEvent func(kind, payload string)
}

// ChatOpener opens a chat's harness session.
type ChatOpener func(ctx context.Context, spec ChatSpec) (ChatSession, error)

// checkChat refuses a chat the runner's own rules don't allow, whatever the server
// says: its ids must be UUIDs, the repo and branch inside the allowlist, and this
// Mac must run a harness with a chat mode.
func (r *JobRunner) checkChat(job *ClaimedJob) error {
	if !jobIDPattern.MatchString(job.ID) || !jobIDPattern.MatchString(job.ChatID) {
		return fmt.Errorf("%w: invalid chat id", ErrRefused)
	}
	if r.Mode != nil && r.Mode() != ModeClaude {
		return fmt.Errorf("%w: this Mac's mode is off", ErrRefused)
	}
	if _, ok := r.Harness.(OpenCode); !ok {
		return fmt.Errorf("%w: this Mac runs Claude Code, which has no chat mode", ErrRefused)
	}
	allowed, err := r.Allowed()
	if err != nil {
		return fmt.Errorf("read the repo allowlist: %w", err)
	}
	return Validate(Job{
		Repo:       job.RepoOwner + "/" + job.RepoName,
		Branch:     job.Branch,
		BaseBranch: job.DefaultBranch,
	}, allowed)
}

// runChat bridges a claimed chat to a harness session until the person stops it,
// the server drops it or the runner shuts down, then reports what it left. The
// chat job holds its slot meanwhile; the server finishes it when the chat stops.
func (r *JobRunner) runChat(ctx context.Context, job *ClaimedJob) {
	api, ok := r.API.(ChatAPI)
	if !ok {
		r.Log.Error("the server client cannot run chats", "chat", job.ChatID)
		return
	}
	secrets := append(r.secrets(), r.chatSecrets(job)...)
	end := func(report string) {
		// Told even when the runner is shutting down: the server would otherwise
		// wait five minutes to call the chat lost.
		done, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
		defer cancel()
		report = redact(report, redactions(secrets))
		if err := api.EndChat(done, r.Token, job.ChatID, report); err != nil {
			r.Log.Warn("could not end the chat", "chat", job.ChatID, "error", err)
		}
	}
	if err := r.checkChat(job); err != nil {
		end("Refused by the runner: " + err.Error())
		return
	}
	if err := r.checkCredential(); err != nil {
		end("The chat could not start: " + err.Error())
		return
	}
	r.Log.Info("chat claimed", "chat", job.ChatID, "repo", job.RepoOwner+"/"+job.RepoName)

	b := &chatBridge{
		r: r, api: api, job: job, redactions: redactions(secrets),
		sleep: r.sleeper(), sess: nil, mu: sync.Mutex{}, pending: nil,
		flushMu: sync.Mutex{},
	}
	sess, err := r.openChat(ctx, job, b.onEvent)
	if err != nil {
		end("The chat could not start: " + err.Error())
		return
	}
	b.sess = sess
	end(b.run(ctx))
}

// chatSecrets are the values a chat's transcript must not carry: the job's board
// account, which the egress proxy holds for the VM.
func (*JobRunner) chatSecrets(job *ClaimedJob) []string {
	return []string{job.MCPClientSecret}
}

func (r *JobRunner) sleeper() func(context.Context, time.Duration) {
	if r.Sleep != nil {
		return r.Sleep
	}
	return sleepCtx
}

func (r *JobRunner) openChat(
	ctx context.Context,
	job *ClaimedJob,
	onEvent func(kind, payload string),
) (ChatSession, error) {
	spec := ChatSpec{
		Job: job, Token: "", Model: "", MCP: nil, Instructions: chatInstructions,
		OnEvent: onEvent,
	}
	if oc, ok := r.Harness.(OpenCode); ok {
		spec.Model = oc.Model
	}
	// The token is the host's: wardcase clones with it and it never enters the VM.
	token, err := r.API.GetPushToken(ctx, r.Token, job.ID)
	if err != nil && !PushUnavailable(err) {
		return nil, fmt.Errorf("get the clone token: %w", err)
	}
	spec.Token = token
	if r.MCP != nil {
		servers, missing, mcpErr := r.MCP()
		if mcpErr != nil {
			return nil, fmt.Errorf("read the device's MCP servers: %w", mcpErr)
		}
		if len(missing) > 0 {
			r.Log.Warn("MCP servers not signed in", "servers", strings.Join(missing, ", "))
		}
		spec.MCP = servers
	}
	open := r.OpenChat
	if open == nil {
		open = r.openWardcaseChat
	}
	return open(ctx, spec)
}

// chatBridge carries one chat: the person's messages in, the harness's events out.
type chatBridge struct {
	r          *JobRunner
	api        ChatAPI
	job        *ClaimedJob
	sess       ChatSession
	redactions []string
	sleep      func(context.Context, time.Duration)

	mu      sync.Mutex
	pending []ChatEvent
	// flushMu keeps a batch's writes in the order the events came.
	flushMu sync.Mutex
}

// onEvent queues one harness event for the server.
func (b *chatBridge) onEvent(kind, payload string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = append(b.pending, ChatEvent{
		Kind: kind, Payload: redact(payload, b.redactions),
	})
}

// take empties the queue.
func (b *chatBridge) take() []ChatEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.pending
	b.pending = nil
	return out
}

// flush sends the queued events, and state when it is not empty, in order. A
// batch that cannot be sent is lost only when ctx ends.
func (b *chatBridge) flush(ctx context.Context, link *chatLink, state string) error {
	b.flushMu.Lock()
	defer b.flushMu.Unlock()
	events := b.take()
	for len(events) > maxEventsPerBatch {
		if err := link.write(ctx, ChatUpdate{
			State: "", Events: events[:maxEventsPerBatch],
		}); err != nil {
			return err
		}
		events = events[maxEventsPerBatch:]
	}
	if len(events) == 0 && state == "" {
		return nil
	}
	return link.write(ctx, ChatUpdate{State: state, Events: events})
}

// run is the chat's life: it returns the report once the chat ended.
func (b *chatBridge) run(ctx context.Context) string {
	bridgeCtx, stop := context.WithCancel(ctx)
	defer stop()
	link := newChatLink(b.api, b.r.Token, b.job.ChatID, b.sleep, b.r.Log)
	inbox := make(chan ChatFrame, chatQueue)
	linkDone := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() { linkDone <- link.pump(bridgeCtx, inbox) })

	stopped := make(chan struct{}, 1)
	wg.Go(func() { b.checkIns(bridgeCtx, stopped) })
	wg.Go(func() { b.flushLoop(bridgeCtx, link) })

	var (
		turn  chan struct{}
		queue []string
		why   string
	)
loop:
	for {
		select {
		case f := <-inbox:
			switch f.Kind {
			case frameStopped:
				break loop
			case frameMessage, frameAnswer:
				queue = append(queue, f.Payload)
			}
		case <-turn:
			turn = nil
		case err := <-linkDone:
			why = "the server connection was lost for good: " + err.Error()
			if ChatGone(err) {
				why = "the server no longer lists this chat for this runner"
			}
			break loop
		case <-stopped:
			break loop
		case <-ctx.Done():
			why = "the runner shut down"
			break loop
		}
		if turn == nil && len(queue) > 0 {
			turn = b.startTurn(bridgeCtx, link, queue[0])
			queue = queue[1:]
		}
	}

	if turn != nil {
		_ = b.sess.Cancel()
		select {
		case <-turn:
		case <-time.After(chatCancelWait):
		}
	}
	// The events the last turn left go out while the server still takes them.
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	_ = b.flush(flushCtx, link, "")
	cancel()
	stop()
	wg.Wait()
	closeCtx, cancelClose := context.WithTimeout(context.WithoutCancel(ctx), closeWait)
	defer cancelClose()
	return chatReport(b.sess.Close(closeCtx), why)
}

// startTurn prompts the harness in the background, so the person's stop is still
// heard while it works, and returns a channel closed when the turn ended.
func (b *chatBridge) startTurn(
	ctx context.Context,
	link *chatLink,
	text string,
) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := b.sess.Prompt(ctx, text); err != nil && ctx.Err() == nil {
			b.onEvent(eventError, err.Error())
		}
		if err := b.flush(ctx, link, stateIdle); err != nil && ctx.Err() == nil {
			b.r.Log.Warn("could not report the turn", "chat", b.job.ChatID, "error", err)
		}
	}()
	return done
}

// flushLoop forwards the events of a turn while it runs.
func (b *chatBridge) flushLoop(ctx context.Context, link *chatLink) {
	t := time.NewTicker(chatFlushEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = b.flush(ctx, link, "")
		case <-ctx.Done():
			return
		}
	}
}

// checkIns reports the chat's job alive, and signals stopped when the server says
// the job is over: the chat was stopped or lost.
func (b *chatBridge) checkIns(ctx context.Context, stopped chan<- struct{}) {
	every := b.r.CheckInEvery
	if every <= 0 {
		every = CheckInEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			stop, err := b.r.API.CheckIn(ctx, b.r.Token, b.job.ID, "")
			if err != nil {
				b.r.Log.Debug("chat check-in failed", "chat", b.job.ChatID, "error", err)
				continue
			}
			if stop {
				select {
				case stopped <- struct{}{}:
				default:
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// chatReport says what the session left behind; why, when set, is why the runner
// ended a chat the person did not stop.
func chatReport(left ChatLeft, why string) string {
	var parts []string
	if why != "" {
		parts = append(parts, "The runner ended the chat: "+why+".")
	}
	if left.Uncommitted == 0 && left.Unpushed == 0 {
		parts = append(parts, "Nothing was left on branch "+left.Branch+".")
	} else {
		parts = append(parts, fmt.Sprintf(
			"Branch %s was left with %d uncommitted file(s) and %d unpushed commit(s) in %s.",
			left.Branch, left.Uncommitted, left.Unpushed, left.Path))
	}
	return strings.Join(parts, " ")
}

// chatLink is a chat's socket to the server. It redials with the last seq it saw,
// and hands writers the live connection, waiting while there is none.
type chatLink struct {
	api     ChatAPI
	token   string
	chatID  string
	sleep   func(context.Context, time.Duration)
	log     *slog.Logger
	now     func() time.Time
	mu      sync.Mutex
	conn    ChatConn
	changed chan struct{}
	cursor  int
}

func newChatLink(
	api ChatAPI,
	token, chatID string,
	sleep func(context.Context, time.Duration),
	log *slog.Logger,
) *chatLink {
	return &chatLink{
		api: api, token: token, chatID: chatID, sleep: sleep, log: log,
		now: time.Now, mu: sync.Mutex{}, conn: nil, changed: make(chan struct{}),
		cursor: 0,
	}
}

func (l *chatLink) state() (ChatConn, <-chan struct{}, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.conn, l.changed, l.cursor
}

func (l *chatLink) setConn(c ChatConn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conn = c
	close(l.changed)
	l.changed = make(chan struct{})
}

// drop forgets c when it is still the live connection.
func (l *chatLink) drop(c ChatConn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == c {
		l.conn = nil
	}
}

// fresh reports whether seq is past the cursor, and moves the cursor there: the
// server may repeat what a replay just sent.
func (l *chatLink) fresh(seq int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if seq <= l.cursor {
		return false
	}
	l.cursor = seq
	return true
}

// write sends the update on the live connection, waiting for the next one when it
// fails, until ctx ends.
func (l *chatLink) write(ctx context.Context, u ChatUpdate) error {
	for {
		conn, changed, _ := l.state()
		if conn != nil {
			err := conn.Write(ctx, u)
			if err == nil {
				return nil
			}
			l.drop(conn)
			conn.Close()
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return fmt.Errorf("send the chat update: %w", ctx.Err())
		}
	}
}

// pump keeps a socket open and forwards the person's messages to inbox, each
// once, in order. It returns when ctx ends, the server refuses the runner for
// good, or it has had no socket for chatGiveUp.
func (l *chatLink) pump(ctx context.Context, inbox chan<- ChatFrame) error {
	backoff := minLoop
	downSince := l.now()
	for ctx.Err() == nil {
		_, _, after := l.state()
		conn, err := l.api.DialChat(ctx, l.token, l.chatID, after)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			if ChatGone(err) {
				return err
			}
			if l.now().Sub(downSince) > chatGiveUp {
				return fmt.Errorf("no connection for %s: %w", chatGiveUp, err)
			}
			l.log.Warn("the chat socket failed; retrying", "chat", l.chatID,
				"error", err, "retry_in", backoff)
			l.sleep(ctx, backoff)
			backoff = min(backoff*backoffFactor, maxBackoff)
			continue
		}
		backoff = minLoop
		l.setConn(conn)
		l.read(ctx, conn, inbox)
		l.drop(conn)
		conn.Close()
		downSince = l.now()
	}
	return ctx.Err() //nolint:wrapcheck // the caller reads it as the end
}

// read forwards frames until the connection fails.
func (l *chatLink) read(ctx context.Context, conn ChatConn, inbox chan<- ChatFrame) {
	for {
		f, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if f.Kind == "" || f.Kind == frameState || (f.Seq > 0 && !l.fresh(f.Seq)) {
			continue
		}
		select {
		case inbox <- f:
		case <-ctx.Done():
			return
		}
	}
}
