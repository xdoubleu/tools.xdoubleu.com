package watchparty_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/watchparty"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var userID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func newTestApp() (*watchparty.WatchParty, http.Handler) {
	cfg := testhelper.NewTestConfig()

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
