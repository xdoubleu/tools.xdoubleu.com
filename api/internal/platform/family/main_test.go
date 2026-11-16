package family_test

import (
	"context"
	"os"
	"testing"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/kit/testhelper"
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
		`CREATE TABLE IF NOT EXISTS global.families (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS global.family_members (
			user_id TEXT PRIMARY KEY,
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS global.app_users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL,
			last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
			role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			display_name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.family_invites (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			from_user_id TEXT NOT NULL,
			to_user_id TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, stmt := range stmts {
		if _, err := postgresDB.Exec(ctx, stmt); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}
