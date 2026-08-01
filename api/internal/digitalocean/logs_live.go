package digitalocean

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"time"

	"github.com/coder/websocket"
)

// liveLogDeadline hard-bounds one component's live log read. DigitalOcean
// closes the socket itself once it has replayed tail_lines (follow is off),
// but a socket that never closes must not hold a Connect request open.
//
// Kept well under DigitalOcean App Platform's own ~25s edge request timeout
// (issue #672): concurrent live reads for a running component were hitting
// the old 20s fallback, pushing total wall-clock past the edge, which then
// reset the upstream connection (503 UC) before any byte was written — a
// silent failure with no log line or Sentry event. The read still degrades
// to whatever text arrived before the deadline, so a lower bound only costs
// a stuck socket, never a healthy one (DO replays the backlog immediately on
// connect). A var, not a const, so tests can drive the deadline path fast.
//
//nolint:gochecknoglobals // test seam, mirrors backoffBase in client.go
var liveLogDeadline = 8 * time.Second

// liveFrameLimit is the largest single websocket frame accepted. The default
// (32 KB) is a plausible size for one log chunk, so it is raised rather than
// risking a read error mid-stream.
const liveFrameLimit = 1 << 20 // 1 MB

// liveFrame is DigitalOcean's live log envelope: one JSON object per frame
// wrapping a chunk of log text, newlines included.
type liveFrame struct {
	Data string `json:"data"`
}

// fetchLiveContent reads a component's live log tail off DigitalOcean's
// websocket, which is the only source of runtime output for a component that
// is still running — nothing has been archived to a historic URL yet.
//
// It never fails the caller: a dial error, a read error, the byte limit, or
// the deadline all return whatever text was collected with truncated set, so
// a flaky socket can't cost the caller the build/deploy logs it already has.
func (c *client) fetchLiveContent(
	ctx context.Context, rawURL string, limit int,
) (string, bool) {
	if limit <= 0 {
		return "", true
	}

	readCtx, cancel := context.WithTimeout(ctx, liveLogDeadline)
	defer cancel()

	//nolint:exhaustruct,bodyclose // only HTTPClient is overridden; the
	// handshake response is hijacked into conn, which CloseNow closes
	conn, _, err := websocket.Dial(readCtx, liveLogURL(rawURL), &websocket.DialOptions{
		HTTPClient: c.wsClient,
	})
	if err != nil {
		c.logger.WarnContext(ctx, "digitalocean live log dial failed",
			slog.Any("error", err))
		return "", true
	}
	defer func() { _ = conn.CloseNow() }()

	conn.SetReadLimit(liveFrameLimit)
	return c.readLive(readCtx, conn, limit)
}

// readLive drains frames until the server closes the socket, the deadline
// hits, or limit bytes have been collected.
func (c *client) readLive(
	ctx context.Context, conn *websocket.Conn, limit int,
) (string, bool) {
	var content []byte
	truncated := false

	for len(content) < limit {
		_, frame, err := conn.Read(ctx)
		if err != nil {
			if !isCleanClose(err) {
				truncated = true
				c.logger.DebugContext(ctx, "digitalocean live log read ended early",
					slog.Any("error", err))
			}
			break
		}
		content = append(content, liveFrameText(frame)...)
	}

	if len(content) >= limit {
		content = content[:limit]
		truncated = true
	}
	return string(content), truncated
}

// isCleanClose reports whether the read ended because DigitalOcean finished
// replaying the backlog and closed the socket, rather than something going
// wrong mid-stream.
func isCleanClose(err error) bool {
	status := websocket.CloseStatus(err)
	return status == websocket.StatusNormalClosure ||
		status == websocket.StatusGoingAway
}

// liveFrameText unwraps the {"data": "…"} envelope. Anything that doesn't
// parse as one — or that carries no text — is passed through verbatim rather
// than dropped, so a JSON-formatted log line is never mistaken for an
// envelope and swallowed.
func liveFrameText(frame []byte) []byte {
	var parsed liveFrame
	if err := json.Unmarshal(frame, &parsed); err != nil || parsed.Data == "" {
		return frame
	}
	return []byte(parsed.Data)
}

// liveLogURL normalizes DigitalOcean's live_url to a websocket scheme: it is
// handed back as http(s) but is a websocket endpoint (doctl rewrites it the
// same way). Its access token rides in the query string, so the dial carries
// no Authorization header — same as the pre-signed historic URLs.
func liveLogURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if parsed.Scheme == "http" || parsed.Scheme == "ws" {
		parsed.Scheme = "ws"
	} else {
		parsed.Scheme = "wss"
	}
	return parsed.String()
}
