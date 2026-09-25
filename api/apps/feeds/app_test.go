package feeds_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/notifications"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *feeds.Feeds

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

// mockWebFetch is testApp's external-content client.
//
//nolint:gochecknoglobals //needed for tests
var mockWebFetch *mocks.MockWebFetchClient

// appUsersRepo lets tests seed users for the problem-email lookup.
//
//nolint:gochecknoglobals //needed for tests
var appUsersRepo *sharedrepos.AppUsersRepository

// userEmail is userID's address, seeded by ensureGlobalAppUsers.
const userEmail = "feeds-test-user@example.com"

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	cfg.EmailInboundDomain = "mail.example.com"
	cfg.EmailInboundSecret = emailWebhookSecret()
	cfg.ResendAPIKey = "test-resend-key"

	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB
	auth := sharedmocks.NewMockedAuthService(userID)

	ensureGlobalAppUsers(postgresDB)
	appUsersRepo = sharedrepos.NewAppUsersRepository(postgresDB)

	mockWebFetch = mocks.NewMockWebFetchClient()
	// Not-configured mailer; notify tests build their own FeedService.
	testApp = feeds.NewInner(
		auth,
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		mockWebFetch,
		notifications.New(
			context.Background(),
			logging.NewNopLogger(),
			mailer.New("", "", ""),
		),
		appUsersRepo,
	)

	if _, err := postgresDB.Exec(
		context.Background(), "DROP SCHEMA IF EXISTS feeds CASCADE",
	); err != nil {
		panic(err)
	}

	if err := testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// ensureGlobalAppUsers mirrors cmd/api's global migrations so these tests can
// run first, then seeds userID with an email.
func ensureGlobalAppUsers(db postgres.DB) {
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
		`CREATE TABLE IF NOT EXISTS global.app_access (
			user_id  TEXT NOT NULL,
			app_name TEXT NOT NULL,
			PRIMARY KEY (user_id, app_name)
		)`,
		// Mirrors cmd/api/migrations/00005_observability.sql for jobqueue.
		`CREATE TABLE IF NOT EXISTS global.job_runs (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			job_id TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			duration_ms BIGINT NOT NULL,
			success BOOLEAN NOT NULL,
			error TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(ctx, stmt); err != nil {
			panic(err)
		}
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO global.app_users (id, email)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email
	`, userID, userEmail); err != nil {
		panic(err)
	}
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Feeds", testApp.GetDisplayName())
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "feeds", testApp.GetName())
}

func TestStart(t *testing.T) {
	assert.NoError(t, testApp.Start())
}
