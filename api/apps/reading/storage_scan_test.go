package reading_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunStorageScanNow_Success covers the manual-trigger path used by the
// admin observability RPC: it wraps the same job the daily scheduled scan
// runs, so a successful run against the fake object store must complete
// without error.
func TestRunStorageScanNow_Success(t *testing.T) {
	err := testApp.RunStorageScanNow(context.Background())
	require.NoError(t, err)
}
