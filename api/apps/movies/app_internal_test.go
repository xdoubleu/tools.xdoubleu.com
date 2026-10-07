package movies

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/testhelper"
)

func TestNewTMDBClient_OnlyWithKey(t *testing.T) {
	cfg := testhelper.NewTestConfig()

	cfg.TMDBAPIKey = ""
	assert.Nil(t, newTMDBClient(logging.NewNopLogger(), cfg))

	cfg.TMDBAPIKey = "token"
	assert.NotNil(t, newTMDBClient(logging.NewNopLogger(), cfg))
}
