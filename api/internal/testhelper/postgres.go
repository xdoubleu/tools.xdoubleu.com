package testhelper

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

const (
	testDBMaxConns         = 25
	testDBConnectTimeout   = 5
	testDBSleepBeforeRetry = 15 * time.Second
	testDBMaxRetryDuration = 30 * time.Second
)

// ConnectTestDB connects to PostgreSQL using standard test parameters.
func ConnectTestDB(dsn string) *pgxpool.Pool {
	db, err := postgres.Connect(
		logging.NewNopLogger(),
		dsn,
		testDBMaxConns,
		"15m",
		testDBConnectTimeout,
		testDBSleepBeforeRetry,
		testDBMaxRetryDuration,
	)
	if err != nil {
		panic(err)
	}
	return db
}
