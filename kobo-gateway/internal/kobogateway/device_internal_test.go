//nolint:exhaustruct //test fixtures only set the fields each case exercises
package kobogateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteConfFileTmpWriteFailure makes the conf dir read-only after the
// conf file exists, so the temp-file write fails.
func TestWriteConfFileTmpWriteFailure(t *testing.T) {
	volumePath := t.TempDir()
	confDir := filepath.Join(volumePath, ".kobo", "Kobo")
	require.NoError(t, os.MkdirAll(confDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(confDir, "Kobo eReader.conf"), []byte(""), 0o644,
	))

	require.NoError(t, os.Chmod(confDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(confDir, 0o755) })

	err := writeConfFile(volumePath, &Conf{})

	assert.Error(t, err)
}

// TestWriteConfFileRenameFailureCleansUpTmp makes eReader.conf a non-empty
// directory so the rename fails, and checks the temp file is removed.
func TestWriteConfFileRenameFailureCleansUpTmp(t *testing.T) {
	volumePath := t.TempDir()
	confDir := filepath.Join(volumePath, ".kobo", "Kobo")
	require.NoError(t, os.MkdirAll(confDir, 0o755))

	occupiedConfPath := filepath.Join(confDir, "Kobo eReader.conf")
	require.NoError(t, os.MkdirAll(occupiedConfPath, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(occupiedConfPath, "occupied"), []byte("x"), 0o644,
	))

	err := writeConfFile(volumePath, &Conf{})

	assert.Error(t, err)

	_, statErr := os.Stat(occupiedConfPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr), "tmp file must be cleaned up on rename failure")
}
