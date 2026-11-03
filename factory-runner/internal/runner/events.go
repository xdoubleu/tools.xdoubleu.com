package runner

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const (
	// maxEventsPerBatch matches what the server accepts in one call.
	maxEventsPerBatch = 200
	// maxQueuedEvents bounds the lines waiting to be sent; the oldest go first.
	maxQueuedEvents = 2000
	// maxJobEvents is how many lines the server keeps for a job.
	maxJobEvents = 5000
	// maxEventBytes is a line's size after redaction; the server cuts at 32 KiB.
	maxEventBytes = 32 * 1024
	redacted      = "[redacted]"
	// EventsEvery is how often forwarded lines are sent.
	EventsEvery   = 2 * time.Second
	appendTimeout = 10 * time.Second
)

// EventLine is one line of harness output with the number the server keeps it
// under; a retried batch repeats the numbers, so nothing is stored twice.
type EventLine struct {
	Seq  int    `json:"seq"`
	Line string `json:"line"`
}

// redactions lists what to search for: each secret as is and as a JSON string
// holds it (the lines are JSON, so a quote, backslash or newline is escaped).
// Values too short to search for are skipped, as in the host checks.
func redactions(secrets []string) []string {
	var out []string
	for _, secret := range secrets {
		if len(secret) < minSecretLen {
			continue
		}
		out = append(out, secret)
		for _, escapeHTML := range []bool{false, true} {
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			enc.SetEscapeHTML(escapeHTML)
			if enc.Encode(secret) == nil {
				quoted := strings.TrimSuffix(b.String(), "\n")
				if inner := quoted[1 : len(quoted)-1]; inner != secret {
					out = append(out, inner)
				}
			}
		}
	}
	return out
}

func redact(line string, values []string) string {
	for _, v := range values {
		line = strings.ReplaceAll(line, v, redacted)
	}
	return line
}

// eventTap turns the harness's output into complete, redacted, numbered lines
// waiting to be forwarded to the server.
type eventTap struct {
	redactions []string

	mu      sync.Mutex
	partial []byte
	// discarding drops the rest of a line that outgrew maxTapLine: its tail must
	// not be sent on its own, where a secret cut in half would no longer match.
	discarding bool
	next       int
	queue      []EventLine
}

func newEventTap(secrets []string) *eventTap {
	return &eventTap{
		redactions: redactions(secrets), mu: sync.Mutex{}, partial: nil,
		discarding: false, next: 1, queue: nil,
	}
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
		raw := t.partial[:i]
		t.partial = t.partial[i+1:]
		if t.discarding {
			t.discarding = false
			continue
		}
		t.add(string(raw))
	}
	if len(t.partial) > maxTapLine {
		t.partial, t.discarding = nil, true
	}
	if over := len(t.queue) - maxQueuedEvents; over > 0 {
		t.queue = t.queue[over:]
	}
	return len(p), nil
}

// add queues one complete line: redacted before it is cut, so a cut can't leave
// half a secret behind.
func (t *eventTap) add(raw string) {
	line := strings.TrimSpace(strings.ToValidUTF8(raw, ""))
	line = strings.ReplaceAll(line, "\x00", "") // Postgres text can't hold it
	if line == "" || t.next > maxJobEvents {
		return
	}
	line = redact(line, t.redactions)
	if len(line) > maxEventBytes {
		line = strings.ToValidUTF8(line[:maxEventBytes], "")
	}
	t.queue = append(t.queue, EventLine{Seq: t.next, Line: line})
	t.next++
}

// take removes and returns up to maxEventsPerBatch queued lines.
func (t *eventTap) take() []EventLine {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := min(len(t.queue), maxEventsPerBatch)
	batch := append([]EventLine(nil), t.queue[:n]...)
	t.queue = t.queue[n:]
	return batch
}

// putBack returns an unsent batch to the front of the queue.
func (t *eventTap) putBack(batch []EventLine) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queue = append(append([]EventLine(nil), batch...), t.queue...)
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
