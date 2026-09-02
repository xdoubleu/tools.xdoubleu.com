package mealplans_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *mealplans.MealPlans

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

	testApp = mealplans.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		familyRepo,
	)

	recipesApp := recipes.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
		familyRepo,
	)

	var err error

	// Drop both schemas so migrations run fresh and in the correct order.
	if _, err = postgresDB.Exec(context.Background(),
		"DROP SCHEMA IF EXISTS mealplans CASCADE; DROP SCHEMA IF EXISTS recipes CASCADE",
	); err != nil {
		panic(err)
	}

	// Ensure global.contacts/families exist (families used by family_id
	// scoping, contacts kept around for any other app's fixtures sharing
	// this schema in CI's single test database).
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
		);
		CREATE TABLE IF NOT EXISTS global.families (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS global.family_members (
			user_id TEXT PRIMARY KEY,
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		panic(err)
	}

	if err = recipesApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
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
	assert.Equal(t, "Meal Plans", testApp.GetDisplayName())
}
