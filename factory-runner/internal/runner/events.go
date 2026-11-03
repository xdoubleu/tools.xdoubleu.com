package runner

import (
	"bytes"
	"cmp"
	"context"
	"strings"
	"sync"
	"time"
)

const (
	// maxEventsPerBatch matches what the server accepts in one call.
	maxEventsPerBatch = 200
	// maxQueuedEvents bounds the lines waiting to be sent; the oldest go first.
	maxQueuedEvents = 2000
	// maxEventBytes is a line's size after redaction; the server cuts at 32 KiB.
	maxEventBytes = 32 * 1024
	redacted      = "[redacted]"
	// EventsEvery is how often forwarded lines are sent.
	EventsEvery   = 2 * time.Second
	appendTimeout = 10 * time.Second
)

// redact replaces every secret value in line. Values too short to search for
// are skipped, as in the host checks.
func redact(line string, secrets []string) string {
	for _, secret := range secrets {
		if len(secret) >= minSecretLen {
			line = strings.ReplaceAll(line, secret, redacted)
		}
	}
	return line
}

// eventTap turns the harness's output into complete, redacted lines waiting to
// be forwarded to the server.
type eventTap struct {
	secrets []string

	mu      sync.Mutex
	partial []byte
	queue   []string
}

// Write implements io.Writer. It never fails: the log is best effort.
func (t *eventTap) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partial = append(t.partial, p...)
	for {
		i := bytes.IndexByte(t.partial, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(t.partial[:i]))
		t.partial = t.partial[i+1:]
		if line == "" {
			continue
		}
		line = redact(strings.ToValidUTF8(line, ""), t.secrets)
		if len(line) > maxEventBytes {
			line = line[:maxEventBytes]
			line = strings.ToValidUTF8(line, "")
		}
		t.queue = append(t.queue, line)
	}
	if len(t.partial) > maxTapLine {
		t.partial = nil
	}
	if over := len(t.queue) - maxQueuedEvents; over > 0 {
		t.queue = t.queue[over:]
	}
	return len(p), nil
}

// take removes and returns up to maxEventsPerBatch queued lines.
func (t *eventTap) take() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := min(len(t.queue), maxEventsPerBatch)
	batch := append([]string(nil), t.queue[:n]...)
	t.queue = t.queue[n:]
	return batch
}

// putBack returns an unsent batch to the front of the queue.
func (t *eventTap) putBack(batch []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queue = append(append([]string(nil), batch...), t.queue...)
	if over := len(t.queue) - maxQueuedEvents; over > 0 {
		t.queue = t.queue[over:]
	}
}

// flush sends everything queued, a batch at a time, and stops at the first
// failure, keeping the rest for the next flush.
func (r *JobRunner) flush(ctx context.Context, jobID string, tap *eventTap) {
	for {
		batch := tap.take()
		if len(batch) == 0 {
			return
		}
		callCtx, cancel := context.WithTimeout(ctx, appendTimeout)
		err := r.API.AppendJobEvents(callCtx, r.Token, jobID, batch)
		cancel()
		if err != nil {
			tap.putBack(batch)
			if ctx.Err() == nil {
				r.Log.Debug("forwarding the job log failed", "job", jobID, "error", err)
			}
			return
		}
	}
}

// forwardEvents sends the job's log every EventsEvery until ctx ends. The caller
// flushes once more after the sandbox has exited.
func (r *JobRunner) forwardEvents(ctx context.Context, jobID string, tap *eventTap) {
	ticker := time.NewTicker(cmp.Or(r.EventsEvery, EventsEvery))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.flush(ctx, jobID, tap)
		}
	}
}
