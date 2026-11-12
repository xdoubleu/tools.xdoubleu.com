package mcptools

import (
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LogSlowRequests warns about MCP requests taking slowCallThreshold or
// longer, naming each JSON-RPC method in the body and the response's
// Content-Type (SSE or JSON; empty for a 202): the request log names only the
// MCP route, and BoundToolCalls sees only tool calls.
func LogSlowRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return logSlowRequests(logger, slowCallThreshold, next)
}

func logSlowRequests(
	logger *slog.Logger, threshold time.Duration, next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The SDK reads the body itself; keep a copy of what it reads.
		var body bytes.Buffer
		r.Body = struct {
			io.Reader
			io.Closer
		}{io.TeeReader(r.Body, &body), r.Body}

		start := time.Now()
		next.ServeHTTP(w, r)
		if took := time.Since(start); took >= threshold {
			logger.WarnContext(r.Context(), "slow MCP request",
				slog.String("methods", strings.Join(jsonRPCMethods(body.Bytes()), ",")),
				slog.String("content_type", w.Header().Get("Content-Type")),
				slog.Duration("duration", took),
			)
		}
	})
}

// jsonRPCMethods names each message of a single or batch JSON-RPC body;
// "response" stands for a message without a method.
func jsonRPCMethods(body []byte) []string {
	type message struct {
		Method string `json:"method"`
	}
	var batch []message
	if json.Unmarshal(body, &batch) != nil {
		var one message
		if json.Unmarshal(body, &one) != nil {
			return []string{"unparsed"}
		}
		batch = []message{one}
	}
	methods := make([]string, len(batch))
	for i, m := range batch {
		methods[i] = cmp.Or(m.Method, "response")
	}
	return methods
}
