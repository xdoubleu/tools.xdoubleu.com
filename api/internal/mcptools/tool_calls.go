package mcptools

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// slowCallThreshold matches Grafana's RequestP95High threshold.
	slowCallThreshold = 5 * time.Second
	// answerSlack is what a tool call leaves the SDK to write its result: the
	// SDK ends the response once the request's own deadline passes.
	answerSlack = time.Second
)

type requestDeadlineKey struct{}

// KeepRequestDeadline hands the request's deadline on to BoundToolCalls: the
// SDK runs tool calls under a context without it.
func KeepRequestDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deadline, ok := r.Context().Deadline(); ok {
			r = r.WithContext(
				context.WithValue(r.Context(), requestDeadlineKey{}, deadline),
			)
		}
		next.ServeHTTP(w, r)
	})
}

// BoundToolCalls is server middleware that runs each tool call under the
// deadline KeepRequestDeadline handed on, less answerSlack, and warns about
// calls taking slowCallThreshold or longer, naming the tool: the request log
// names only the MCP route.
func BoundToolCalls(logger *slog.Logger) mcp.Middleware {
	return boundToolCalls(logger, slowCallThreshold, answerSlack)
}

func boundToolCalls(
	logger *slog.Logger, threshold, slack time.Duration,
) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			call, ok := req.(*mcp.CallToolRequest)
			if !ok || call.Params == nil {
				return next(ctx, method, req)
			}
			if deadline, ok := ctx.Value(requestDeadlineKey{}).(time.Time); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline.Add(-slack))
				defer cancel()
			}

			start := time.Now()
			res, err := next(ctx, method, req)
			if took := time.Since(start); took >= threshold {
				logger.WarnContext(ctx, "slow MCP tool call",
					slog.String("tool", call.Params.Name),
					slog.Duration("duration", took),
					slog.Bool("expired", errors.Is(ctx.Err(), context.DeadlineExceeded)),
				)
			}
			return res, err
		}
	}
}
