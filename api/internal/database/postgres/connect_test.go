package postgres_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
)

func TestConnectRetriesThenFailsOnBadDSN(t *testing.T) {
	t.Parallel()

	_, err := postgres.Connect(
		logging.NewNopLogger(),
		"postgres://postgres@127.0.0.1:1/nope",
		1,
		"1m",
		1,
		10*time.Millisecond,
		50*time.Millisecond,
	)
	require.Error(t, err)
}

func TestConnectRejectsInvalidDSN(t *testing.T) {
	t.Parallel()

	_, err := postgres.Connect(
		logging.NewNopLogger(),
		"://not a url",
		1,
		"1m",
		1,
		time.Millisecond,
		time.Millisecond,
	)
	assert.Error(t, err)
}
