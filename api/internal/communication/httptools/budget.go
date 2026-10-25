package httptools

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	// RequestBudget is every request's context deadline. The server's
	// WriteTimeout must exceed it by WriteSlack: a response written after
	// WriteTimeout is cut with no log.
	RequestBudget = 8 * time.Second
	// LongRequestBudget replaces RequestBudget on routes that legitimately take
	// longer; their write deadline moves with it. It plus WriteSlack must stay
	// under kamal-proxy's response_timeout.
	LongRequestBudget = 25 * time.Second
	// WriteSlack is the time left to write a response once its budget fires.
	WriteSlack = 2 * time.Second
	// UpstreamTimeout bounds one request-path call to an external service,
	// leaving the handler time to answer when it gives up.
	UpstreamTimeout = RequestBudget * 3 / 4
)

// Budget gives each request a RequestBudget context deadline, or
// LongRequestBudget plus a matching write deadline where long reports true.
// WebSocket upgrades get neither: hijacking clears the write deadline and the
// socket outlives any budget.
func Budget(
	logger *slog.Logger,
	long func(*http.Request) bool,
) func(http.Handler) http.Handler {
	return budget(logger, long, RequestBudget, LongRequestBudget)
}

func budget(
	logger *slog.Logger,
	long func(*http.Request) bool,
	short, longer time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, r)
				return
			}

			d := short
			if long(r) {
				d = longer
				err := http.NewResponseController(w).
					SetWriteDeadline(time.Now().Add(d + WriteSlack))
				if err != nil {
					logger.ErrorContext(r.Context(), "cannot extend write deadline",
						slog.String("path", r.URL.Path), slog.Any("error", err))
				}
			}

			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))

			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logger.WarnContext(ctx, "request exceeded its budget",
					slog.String("path", r.URL.Path), slog.Duration("budget", d))
			}
		})
	}
}

// Detach drops ctx's cancellation but keeps its deadline, for work that must
// survive the client leaving yet still fit the request's budget.
func Detach(ctx context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(detached, deadline)
	}
	return detached, func() {}
}
