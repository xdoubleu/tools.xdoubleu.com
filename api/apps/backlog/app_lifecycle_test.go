package backlog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// TestNew_ReturnsApp covers the production New constructor that wires real
// steam/Open Library factories.
func TestNew_ReturnsApp(t *testing.T) {
	bl := backlog.New(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
	)
	require.NotNil(t, bl)
}

// TestStart_RegistersJobs covers Start → setJobs.
func TestStart_RegistersJobs(t *testing.T) {
	bl := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				return nil
			},
			OpenLibrary:      nil,
			GoogleBooks:      nil,
			UniCat:           nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NotNil(t, bl)
	err := bl.Start()
	require.NoError(t, err)
}
