package oauth2as_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func readV049Fixture(t *testing.T) v049Fixture {
	t.Helper()
	raw, err := os.ReadFile(v049FixturePath)
	require.NoError(t, err)
	var f v049Fixture
	require.NoError(t, json.Unmarshal(raw, &f))
	return f
}

// loadV049Fixture writes the fixture's clients and rows, every timestamp moved
// forward by the time since capture so the tokens are live again.
func loadV049Fixture(t *testing.T, db *pgxpool.Pool) v049Fixture {
	t.Helper()
	ctx := context.Background()
	f := readV049Fixture(t)
	shift := time.Since(f.CapturedAt)

	for _, table := range v049Tables {
		key := "signature"
		if table == "oauth2_clients" {
			key = "id"
		}
		for _, raw := range f.Rows[table] {
			var row map[string]any
			require.NoError(t, json.Unmarshal(raw, &row))
			shifted, err := json.Marshal(shiftTimes(row, shift))
			require.NoError(t, err)

			_, err = db.Exec(ctx,
				`DELETE FROM auth.`+table+` WHERE `+key+` = $1`, row[key],
			)
			require.NoError(t, err)
			_, err = db.Exec(ctx, `INSERT INTO auth.`+table+
				` SELECT * FROM jsonb_populate_record(NULL::auth.`+table+`, $1)`,
				shifted,
			)
			require.NoError(t, err)
		}
	}
	return f
}

// shiftTimes moves every non-zero RFC 3339 string in v by shift, keeping its
// layout.
func shiftTimes(v any, shift time.Duration) any {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			val[k] = shiftTimes(child, shift)
		}
	case []any:
		for i, child := range val {
			val[i] = shiftTimes(child, shift)
		}
	case string:
		ts, err := time.Parse(time.RFC3339Nano, val)
		if err != nil || ts.IsZero() {
			return val
		}
		return ts.Add(shift).Format(time.RFC3339Nano)
	}
	return v
}

// backfillSQL returns the Up or Down block of the claim-time backfill.
func backfillSQL(t *testing.T, up bool) string {
	t.Helper()
	paths, err := filepath.Glob(
		"../../cmd/api/migrations/*_oauth2_session_numeric_claim_times.sql",
	)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	raw, err := os.ReadFile(paths[0])
	require.NoError(t, err)

	upPart, downPart, ok := strings.Cut(string(raw), "-- +goose Down")
	require.True(t, ok)
	block := downPart
	if up {
		block = upPart
	}
	var lines []string
	for line := range strings.SplitSeq(block, "\n") {
		if !strings.HasPrefix(line, "-- +goose") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func applyBackfill(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(context.Background(), backfillSQL(t, true))
	require.NoError(t, err)
}

// storedClaims returns the id_token_claims stored under signature.
func storedClaims(
	t *testing.T, db *pgxpool.Pool, table, signature string,
) map[string]any {
	t.Helper()
	var claims map[string]any
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT request #> '{session,id_token_claims}' FROM auth.`+table+
			` WHERE signature = $1`, signature,
	).Scan(&claims))
	return claims
}

// hmacSignature is the stored key of an HMAC token: the part after the dot.
func hmacSignature(t *testing.T, token string) string {
	t.Helper()
	_, sig, ok := strings.Cut(token, ".")
	require.True(t, ok)
	return sig
}
