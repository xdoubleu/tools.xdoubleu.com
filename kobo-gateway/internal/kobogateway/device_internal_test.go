package kobogateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteConfFileTmpWriteFailure exercises writeConfFile's initial-write
// failure branch: the conf directory is made read-only after the conf file
// exists (so the os.Stat precondition still holds) but before the temp file
// write, which needs directory write permission to create a new entry.
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

// TestWriteConfFileRenameFailureCleansUpTmp exercises writeConfFile's
// rename-failure branch: Kobo eReader.conf is a non-empty directory instead
// of a file, so the stat precondition still holds but os.Rename onto it
// fails, and the temp file must be cleaned up rather than left behind.
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
