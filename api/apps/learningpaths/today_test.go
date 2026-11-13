package learningpaths_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths"
	"tools.xdoubleu.com/internal/kit/crypto"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

func TestListActivePaths(t *testing.T) {
	paths, err := testApp.ListActivePaths(context.Background(), userID, 3)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(paths), 3)
}

func TestListActivePaths_StoreFailure(t *testing.T) {
	closed := testhelper.ConnectTestDB(testCfg.DBDsn)
	closed.Close()
	sealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	a := learningpaths.New(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		closed,
		sealer,
		testBooks,
		testFeeds,
	)

	_, err = a.ListActivePaths(context.Background(), userID, 3)

	assert.Error(t, err)
}
