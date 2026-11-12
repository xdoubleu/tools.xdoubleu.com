package movies_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/platform/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
)

func TestListWatching(t *testing.T) {
	const uid = "movies-today-test-user"
	c := newClient(t, uid, fakeTMDB{})
	a := newApp(uid, fakeTMDB{})
	e := add(t, c, "movie", 603, "want")
	_, err := c.SetStatus(context.Background(),
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watching", UnknownDate: false})
	require.NoError(t, err)
	add(t, c, "movie", 129, "want")

	got, err := a.ListWatching(context.Background(), uid, 3)
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, "The Matrix", got[0].Title)
	assert.NotEmpty(t, got[0].ID)
}

func TestListWatching_StoreFailure(t *testing.T) {
	closed := testhelper.ConnectTestDB(testCfg.DBDsn)
	closed.Close()
	a := movies.New(
		context.Background(),
		sharedmocks.NewMockedAuthService("u"),
		logging.NewNopLogger(),
		testCfg,
		closed,
		testFamily,
	)

	_, err := a.ListWatching(context.Background(), "u", 3)

	assert.Error(t, err)
}
