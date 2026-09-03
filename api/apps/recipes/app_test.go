package recipes_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *recipes.Recipes

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var familyRepo *sharedrepositories.FamilyRepository

//nolint:gochecknoglobals //needed for tests
var testCfg config.Config

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB
	familyRepo = sharedrepositories.NewFamilyRepository(postgresDB)

	testApp = recipes.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		familyRepo,
	)

	// Drop the schema so the rewritten migration is applied from scratch.
	var err error
	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS recipes CASCADE",
	); err != nil {
		panic(err)
	}

	// Ensure the global.families tables exist — the apps key their data by
	// family_id and CI runs every package against one shared test database.
	if _, err = postgresDB.Exec(context.Background(), `
		CREATE SCHEMA IF NOT EXISTS global;
		CREATE TABLE IF NOT EXISTS global.families (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS global.family_members (
			user_id TEXT PRIMARY KEY,
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			display_name TEXT NOT NULL DEFAULT ''
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
