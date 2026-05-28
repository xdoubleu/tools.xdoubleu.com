package shoppinglist_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *shoppinglist.ShoppingList

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv

	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB
	auth := sharedmocks.NewMockedAuthService(userID)

	recipesApp := recipes.New(auth, logging.NewNopLogger(), cfg, postgresDB)
	mealPlansApp := mealplans.New(auth, logging.NewNopLogger(), cfg, postgresDB)

	testApp = shoppinglist.New(auth, logging.NewNopLogger(), cfg, postgresDB)

	var err error
	for _, schema := range []string{"shoppinglist", "mealplans", "recipes"} {
		if _, err = postgresDB.Exec(
			context.Background(),
			"DROP SCHEMA IF EXISTS "+schema+" CASCADE",
		); err != nil {
			panic(err)
		}
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
