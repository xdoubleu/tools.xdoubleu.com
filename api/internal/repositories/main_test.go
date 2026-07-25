package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB

	// Mirrors cmd/api/migrations/00001_init.sql, 00005_observability.sql, and
	// 00007_profile_shares_per_app.sql so these tests can run before the
	// cmd/api package has applied the global migrations.
	ctx := context.Background()
	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.app_users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL,
			last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
			role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			display_name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT NOT NULL,
			app TEXT NOT NULL CHECK (app IN ('books', 'games')),
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, app)
		)`,
		`CREATE TABLE IF NOT EXISTS global.usage_daily (
			day DATE NOT NULL,
			app TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			count BIGINT NOT NULL,
			PRIMARY KEY (day, app, endpoint)
		)`,
		`CREATE TABLE IF NOT EXISTS global.storage_snapshots (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			scanned_at TIMESTAMPTZ NOT NULL,
			total_size_bytes BIGINT NOT NULL,
			object_count BIGINT NOT NULL,
			orphan_size_bytes BIGINT NOT NULL,
			orphan_count BIGINT NOT NULL,
			stale_upload_size_bytes BIGINT NOT NULL,
			stale_upload_count BIGINT NOT NULL,
			prefix_breakdown JSONB NOT NULL
		)`,
		// Mirrors cmd/api/migrations/00009_oauth_connections.sql and
		// 00010_oauth_connections_config.sql.
		`CREATE TABLE IF NOT EXISTS global.oauth_connections (
			provider      TEXT PRIMARY KEY
				CHECK (provider IN ('github', 'sentry', 'digitalocean')),
			access_token  BYTEA NOT NULL,
			refresh_token BYTEA,
			expires_at    TIMESTAMPTZ,
			connected_by  TEXT NOT NULL,
			connected_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
			config        JSONB
		)`,
	}
	for _, stmt := range stmts {
		if _, err := postgresDB.Exec(ctx, stmt); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}
