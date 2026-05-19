package watchparty_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/config"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

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
	)

	mux := http.NewServeMux()
	app.Routes(app.GetName(), mux)

	return app, mux
}

func TestGetDisplayName(t *testing.T) {
	app, _ := newTestApp()
	assert.Equal(t, "WatchParty", app.GetDisplayName())
}
