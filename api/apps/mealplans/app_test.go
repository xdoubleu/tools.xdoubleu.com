package mealplans_test

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
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testApp *mealplans.MealPlans

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

	testApp = mealplans.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		postgresDB,
	)

	var err error

	// Ensure recipes schema exists (mealplans migration moves tables from it).
	if _, err = postgresDB.Exec(context.Background(), `
		CREATE SCHEMA IF NOT EXISTS recipes;
		CREATE TABLE IF NOT EXISTS recipes.recipes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			instructions TEXT NOT NULL DEFAULT '',
			base_servings INT NOT NULL DEFAULT 2,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS recipes.plans (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			owner_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ical_token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
			ical_hide_slots TEXT[] NOT NULL DEFAULT '{}',
			ical_hide_past BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS recipes.plan_access (
			plan_id UUID NOT NULL REFERENCES recipes.plans (id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			can_edit BOOL NOT NULL DEFAULT FALSE,
			PRIMARY KEY (plan_id, user_id)
		);
		CREATE TABLE IF NOT EXISTS recipes.plan_meals (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			plan_id UUID NOT NULL REFERENCES recipes.plans (id) ON DELETE CASCADE,
			meal_date DATE NOT NULL,
			meal_slot TEXT NOT NULL CHECK (meal_slot IN ('breakfast', 'noon', 'evening')),
			recipe_id UUID REFERENCES recipes.recipes (id),
			custom_name TEXT NOT NULL DEFAULT '',
			servings INT NOT NULL DEFAULT 2,
			UNIQUE (plan_id, meal_date, meal_slot),
			CONSTRAINT plan_meals_meal_check CHECK (
				recipe_id IS NOT NULL OR custom_name != ''
			)
		);
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

	// Drop mealplans schema so migration runs fresh.
	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS mealplans CASCADE",
	); err != nil {
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
