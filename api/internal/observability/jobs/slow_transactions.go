package jobs

import (
	"context"
	"strings"

	"tools.xdoubleu.com/internal/models"
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
	// are legitimately slow. These gate the trending "currently slow" list on
	// /monitoring/observability and the weekly digest's slow-transaction
	// section; the p95 latency *alert* moved to Grafana on real Prometheus
	// histograms (issue #1528, docs/adr-0011-slow-transaction-thresholds.md).
	slowTransactionHTTPThresholdMs     = 5 * msPerSecond
	slowTransactionFrontendThresholdMs = 5 * msPerSecond
	slowTransactionJobThresholdMs      = 60 * msPerSecond

	msPerSecond = 1_000

	// pctChangeToPercent converts TransactionTrend.PctChange (a 0.20 = +20%
	// fraction) to a whole-number percent for display.
	pctChangeToPercent = 100
)

// slowTransactionExcluded reports whether transaction is a name that would
// otherwise be classified as frontend but isn't a representative page load —
// e.g. NextNodeServer.clientComponentLoading is a Next.js-internal
// transaction that legitimately runs far longer than any real page load
// (61s observed), and would keep the frontend p95 permanently in the "slow"
// list at the same threshold that fits every other frontend transaction
// (issue #1310).
//
// games' and books' "GET /<prefix>/api/progress" WebSocket-upgrade routes
// (apps/games/routes.go, apps/books/routes.go, internal/progressws) are
// deliberately NOT excluded here (issue #1320) even though their
// connection-lifetime transaction will permanently register as slow —
// excluding it left the whole upgrade path with zero latency signal, and
// acceptWithHandshakeSpan
// (internal/communication/wstools/websocket.go) only measures the
// handshake, not this transaction.
func slowTransactionExcluded(transaction string) bool {
	return transaction == "NextNodeServer.clientComponentLoading"
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

// thresholdMsForClass returns the "slow" threshold for a transaction class.
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

// slowTransactionsRepo is the subset of
// *repositories.TransactionLatencyRepository this package needs.
type slowTransactionsRepo interface {
	Trends(ctx context.Context) ([]models.TransactionTrend, error)
}

// currentlySlowTransactions returns the regressing transactions (already
// filtered to a >=20% p95 increase by Trends) whose recent p95 is also at or
// above its class's threshold (classifyTransaction/thresholdMsForClass,
// issue #1310) — a transaction that merely got relatively slower without
// crossing its class's absolute threshold isn't "slow" for notification
// purposes, just for the trending list on /monitoring/observability.
// Excluded transactions (slowTransactionExcluded) never count as slow here.
func currentlySlowTransactions(
	ctx context.Context,
	repo slowTransactionsRepo,
) ([]models.TransactionTrend, error) {
	trends, err := repo.Trends(ctx)
	if err != nil {
		return nil, err
	}

	slow := make([]models.TransactionTrend, 0, len(trends))
	for _, t := range trends {
		if slowTransactionExcluded(t.Transaction) {
			continue
		}
		threshold := thresholdMsForClass(classifyTransaction(t.Transaction))
		if t.RecentAvgP95Ms >= threshold {
			slow = append(slow, t)
		}
	}
	return slow, nil
}
