package reading_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// TestNew_ReturnsApp covers the production New constructor that wires real
// UniCat / Hardcover / R2 clients.
func TestNew_ReturnsApp(t *testing.T) {
	bl := reading.New(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
	)
	require.NotNil(t, bl)
}

// TestStart_RegistersJobs covers Start registering the resync job and topics.
func TestStart_RegistersJobs(t *testing.T) {
	bl := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		reading.Clients{
			UniCat:           nil,
			WebFetch:         nil,
			Arxiv:            nil,
			HTMLConvert:      nil,
			Hardcover:        nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NotNil(t, bl)
	err := bl.Start()
	require.NoError(t, err)
}
