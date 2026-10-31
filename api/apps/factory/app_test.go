package factory_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const adminID = "factory-test-admin"

//nolint:gochecknoglobals //needed for tests
var (
	testDB   postgres.DB
	testPool *pgxpool.Pool
	testCfg  config.Config
)

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = pool
	testPool = pool

	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS factory CASCADE"); err != nil {
		panic(err)
	}
	app := factory.New(
		ctx,
		sharedmocks.NewMockedAdminAuthService(adminID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
	)
	if err := app.ApplyMigrations(ctx, pool); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// newClient serves the app to a caller authenticated as admin through the
// given channel; the board is emptied first.
func newClient(
	t *testing.T,
	admin bool,
	channel string,
) factoryv1connect.TaskServiceClient {
	t.Helper()
	if _, err := testDB.Exec(
		context.Background(), "DELETE FROM factory.tasks",
	); err != nil {
		t.Fatal(err)
	}
	return newClientKeepingBoard(t, admin, channel)
}

// newApp is the app acting as the admin or a plain user, authenticated
// through channel.
func newApp(admin bool, channel string) *factory.Factory {
	return factory.New(
		context.Background(),
		sharedmocks.NewMockedChannelAuthService(adminID, admin, channel),
		logging.NewNopLogger(),
		testCfg,
		testDB,
	)
}

func newTransport(t *testing.T, app *factory.Factory) connect.Transport {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	return connecthttp.NewTransport(
		http.DefaultClient,
		ts.URL,
		connecthttp.WithReadMaxBytes(0),
	)
}

// newClientKeepingBoard is newClient without emptying the board, for a
// second caller on the same tasks.
func newClientKeepingBoard(
	t *testing.T,
	admin bool,
	channel string,
) factoryv1connect.TaskServiceClient {
	t.Helper()
	return factoryv1connect.NewTaskServiceClient(
		connect.NewClient(newTransport(t, newApp(admin, channel))),
	)
}

func newRepoClient(
	t *testing.T,
	admin bool,
	channel string,
) factoryv1connect.RepoServiceClient {
	t.Helper()
	return factoryv1connect.NewRepoServiceClient(
		connect.NewClient(newTransport(t, newApp(admin, channel))),
	)
}

func browserClient(t *testing.T) factoryv1connect.TaskServiceClient {
	t.Helper()
	return newClient(t, true, constants.ChannelBrowser)
}

func code(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return connect.CodeUnknown
}
