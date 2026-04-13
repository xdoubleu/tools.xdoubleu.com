package watchparty_test

import (
	"net/http"
	"os"
	"testing"

	configtools "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/templates"
)

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name:  "accessToken",
	Value: "access",
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func newTestApp() (*watchparty.WatchParty, http.Handler) {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Throttle = false

	app := watchparty.New(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		templates.LoadShared(cfg),
	)

	mux := http.NewServeMux()
	app.Routes(app.GetName(), mux)

	return app, mux
}
