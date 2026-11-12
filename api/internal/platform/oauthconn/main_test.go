package oauthconn_test

import (
	"context"
	"os"
	"testing"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB

	// Mirror the global migrations so these tests run before cmd/api applies them.
	ctx := context.Background()
	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.oauth_connections (
			provider      TEXT PRIMARY KEY
				CHECK (provider IN ('github', 'sentry', 'digitalocean')),
			access_token  BYTEA NOT NULL,
			refresh_token BYTEA,
			expires_at    TIMESTAMPTZ,
			connected_by  TEXT NOT NULL,
			connected_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
			config        JSONB,
			scope         TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := postgresDB.Exec(ctx, stmt); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}
