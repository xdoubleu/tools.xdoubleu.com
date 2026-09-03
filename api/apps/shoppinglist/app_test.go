package shoppinglist_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *shoppinglist.ShoppingList

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var familyRepo *sharedrepositories.FamilyRepository

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()

	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB
	familyRepo = sharedrepositories.NewFamilyRepository(postgresDB)
	auth := sharedmocks.NewMockedAuthService(userID)

	recipesApp := recipes.New(auth, logging.NewNopLogger(), cfg, postgresDB, familyRepo)
	mealPlansApp := mealplans.New(
		auth, logging.NewNopLogger(), cfg, postgresDB, familyRepo,
	)

	testApp = shoppinglist.New(
		auth, logging.NewNopLogger(), cfg, postgresDB, familyRepo,
	)

	var err error
	for _, schema := range []string{"shoppinglist", "mealplans", "recipes"} {
		if _, err = postgresDB.Exec(
			context.Background(),
			"DROP SCHEMA IF EXISTS "+schema+" CASCADE",
		); err != nil {
			panic(err)
		}
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

	if err = recipesApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	if err = mealPlansApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
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
	assert.Equal(t, "Shopping List", testApp.GetDisplayName())
}
