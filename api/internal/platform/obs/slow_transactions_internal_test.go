package obs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/sentryapi"
)

func TestProtoSlowTransactionsDropsRunnerLongPolls(t *testing.T) {
	t.Parallel()

	stat := func(name string, p95 float64) sentryapi.TransactionStat {
		return sentryapi.TransactionStat{
			Transaction: name, Project: "api", P95DurationMs: p95, RequestCount: 1,
		}
	}
	got := protoSlowTransactions([]sentryapi.TransactionStat{
		stat("POST /factory.v1.RunnerService/ClaimJob", 20003),
		stat("POST /factory.v1.RunnerService/Heartbeat", 20001),
		stat("GET /games/api/progress", 900),
	})

	require.Len(t, got, 1)
	assert.Equal(t, "GET /games/api/progress", got[0].GetTransaction())
}
