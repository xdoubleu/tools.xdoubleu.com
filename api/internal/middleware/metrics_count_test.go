package middleware_test

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// requestCount totals http_request_duration_seconds_count for a route and
// status class. The histogram is process-wide, so tests compare the count
// before and after instead of asserting an absolute number, which stays valid
// under -count=N.
func requestCount(t *testing.T, route, status string) int {
	t.Helper()

	re := regexp.MustCompile(
		`(?m)^http_request_duration_seconds_count\{[^}]*route="` +
			regexp.QuoteMeta(route) + `"[^}]*status="` + regexp.QuoteMeta(status) +
			`"[^}]*\} (\d+)$`,
	)

	total := 0
	for _, m := range re.FindAllStringSubmatch(scrapeMetrics(t), -1) {
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)

		total += n
	}

	return total
}
