package recipes_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *recipes.Recipes

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	testCfg = config.New(logging.NewNopLogger())
	testCfg.Env = configtools.TestEnv

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	testApp = recipes.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
	)

	// Drop the schema so the rewritten migration is applied from scratch.
	var err error
	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS recipes CASCADE",
	); err != nil {
		panic(err)
	}

	// Ensure global.contacts exists (used by recipe-book sharing queries).
	if _, err = postgresDB.Exec(context.Background(), `
		CREATE SCHEMA IF NOT EXISTS global;
		CREATE TABLE IF NOT EXISTS global.contacts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			owner_user_id TEXT NOT NULL,
			contact_user_id TEXT NOT NULL,
			display_name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (owner_user_id, contact_user_id)
		)`); err != nil {
		panic(err)
	}

	if err = testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Recipes", testApp.GetDisplayName())
}
