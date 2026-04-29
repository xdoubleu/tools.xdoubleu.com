package todos_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"tools.xdoubleu.com/apps/todos"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

//nolint:gochecknoglobals //needed for tests
var testApp *todos.Todos

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv

	postgresDB, err := postgres.Connect(
		logging.NewNopLogger(),
		cfg.DBDsn,
		25,
		"15m",
		5,
		15*time.Second,
		30*time.Second,
	)
	if err != nil {
		panic(err)
	}
	testDB = postgresDB

	testApp = todos.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		postgresDB,
		templates.LoadShared(cfg),
	)

	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS todos CASCADE",
	); err != nil {
		panic(err)
	}

	if err = testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	mux := http.NewServeMux()
	testApp.Routes(testApp.GetName(), mux)
	return mux
}

func createTask(t *testing.T, input string) string {
	t.Helper()
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title)
		VALUES ($1, $2)`,
		userID, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err = testDB.QueryRow(t.Context(), `
		SELECT id::text FROM todos.tasks
		WHERE owner_user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
