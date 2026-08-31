package jobs

import (
	"context"
	"errors"
	"strings"

	"tools.xdoubleu.com/internal/sentryapi"
)

const (
	// slowTransactionHTTPThresholdMs/-JobThresholdMs/-FrontendThresholdMs
	// are per-class "slow" thresholds (issue #1310) — a single global number
	// doesn't work here: a background job legitimately running tens of
	// seconds (e.g. steam sync at ~24s) is not comparable to an HTTP handler
	// running that long, which would blow straight past httpWriteTimeout
	// (10s, cmd/api/main.go). Typical api RPCs run 0.9-1.5s and typical web
	// page loads 1.1-2.0s, so 5s gives real headroom above normal for both
	// while staying well under the handler's hard write-timeout ceiling.
	// Background jobs get a much longer allowance since some (steam sync)
	// are legitimately slow and the rule would otherwise fire on rollout.
	slowTransactionHTTPThresholdMs     = 5 * msPerSecond
	slowTransactionFrontendThresholdMs = 5 * msPerSecond
	slowTransactionJobThresholdMs      = 60 * msPerSecond

	msPerSecond = 1_000
)

// slowTransactionExcluded reports whether transaction is a name that would
// otherwise be classified as frontend or an HTTP handler but isn't
// comparable to the rest of its class — e.g.
// NextNodeServer.clientComponentLoading is a Next.js-internal transaction
// that legitimately runs far longer than any real page load (61s observed),
// and would trip slow_transaction_frontend_high permanently at the same
// threshold that fits every other frontend transaction (issue #1310).
//
// isWebSocketProgressTransaction excludes the equivalent case on the HTTP
// side: games' and books' "GET /<prefix>/api/progress" routes
// (apps/games/routes.go, apps/books/routes.go) are WebSocket upgrades
// (a.Services.WebSocket.Handler(), internal/progressws) rather than bounded
// request/response handlers. sentryhttp's transaction span covers the whole
// handler call, which for an upgraded socket doesn't return until the
// connection closes — so its "duration" measures how long a client kept the
// tab open, not request latency, and would breach slow_transaction_http_high
// (issue #1320) the moment any client session outlives the 5s HTTP
// threshold meant for bounded handlers.
func slowTransactionExcluded(transaction string) bool {
	return transaction == "NextNodeServer.clientComponentLoading" ||
		isWebSocketProgressTransaction(transaction)
}

// isWebSocketProgressTransaction matches the "GET /<prefix>/api/progress"
// shape shared by every app registering a progressws WebSocket handler,
// without hardcoding app names, so a future app adopting the same pattern
// is covered automatically. It does not match the sibling
// ".../api/progress/{id}/refresh" route, which is a normal bounded HTTP
// handler and stays subject to the usual threshold.
func isWebSocketProgressTransaction(transaction string) bool {
	return strings.HasPrefix(transaction, "GET ") &&
		strings.HasSuffix(transaction, "/api/progress")
}

// transactionClass distinguishes the three "shapes" of Sentry transaction
// this repo produces, since a legitimately-slow background job (e.g. steam
// sync at ~24s) is not comparable to an HTTP handler or page load running
// that long (issue #1310).
type transactionClass int

const (
	transactionClassHTTPHandler transactionClass = iota
	transactionClassBackgroundJob
	transactionClassFrontend
)

// thresholdMsForClass returns the "slow" threshold for a transaction class —
// the single seam currentlySlowTransactions (slow_transactions.go) uses to
// replace its own now-removed provisional flat constant with these per-class
// numbers, per issue #1310.
func thresholdMsForClass(class transactionClass) float64 {
	switch class {
	case transactionClassHTTPHandler:
		return slowTransactionHTTPThresholdMs
	case transactionClassFrontend:
		return slowTransactionFrontendThresholdMs
	case transactionClassBackgroundJob:
		return slowTransactionJobThresholdMs
	}
	return slowTransactionJobThresholdMs
}

// classifyTransaction infers a transaction's class from its name shape
// alone — Sentry project names are admin-configured free text (see
// sentryapi.client.resolveConfig), not a fixed enum, so classifying off the
// transaction name is the only stable signal. An HTTP-verb prefix (the verb
// prefixes ConnectRPC/net-http transaction names carry, e.g.
// "GET /games/api/progress", "POST /games.v1.GamesService/RefreshSteamGame")
// is an api handler; a leading "/" or an embedded "." (e.g. a page route or
// a Next.js internal like "NextNodeServer.clientComponentLoading") is a
// frontend transaction; everything else (bare job identifiers like "steam",
// "poll-feeds") is a background job.
func classifyTransaction(transaction string) transactionClass {
	httpMethodPrefixes := []string{"GET ", "POST ", "PUT ", "PATCH ", "DELETE "}
	for _, prefix := range httpMethodPrefixes {
		if strings.HasPrefix(transaction, prefix) {
			return transactionClassHTTPHandler
		}
	}
	if strings.HasPrefix(transaction, "/") || strings.Contains(transaction, ".") {
		return transactionClassFrontend
	}
	return transactionClassBackgroundJob
}

// slowTransactionEvaluator breaches when any non-excluded transaction of the
// given class has a p95 duration exceeding threshold, reporting the highest
// p95 among them — the same "worst offender" pattern as ciDurationEvaluator.
func slowTransactionEvaluator(
	repo transactionStatsLister,
	class transactionClass,
	threshold float64,
) func(context.Context) (float64, bool, error) {
	return func(ctx context.Context) (float64, bool, error) {
		stats, err := repo.ListTransactionStats(ctx)
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}

		maxP95 := 0.0
		for _, s := range stats {
			if slowTransactionExcluded(s.Transaction) {
				continue
			}
			if classifyTransaction(s.Transaction) != class {
				continue
			}
			if s.P95DurationMs > maxP95 {
				maxP95 = s.P95DurationMs
			}
		}
		return maxP95, maxP95 > threshold, nil
	}
}
