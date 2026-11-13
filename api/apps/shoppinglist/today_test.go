package shoppinglist_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

func TestCountCustomListItems(t *testing.T) {
	n, err := testApp.CountCustomListItems(context.Background(), userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 0)
}

func TestCountCustomListItems_StoreFailure(t *testing.T) {
	closed := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	closed.Close()
	a := shoppinglist.New(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testhelper.NewTestConfig(),
		closed,
		familyRepo,
	)

	_, err := a.CountCustomListItems(context.Background(), userID)

	assert.Error(t, err)
}
