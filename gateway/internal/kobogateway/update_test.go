package kobogateway_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

// machO64Header is the magic prefix of a darwin/arm64 binary.
func machO64Header(payload string) []byte {
	return append([]byte{0xcf, 0xfa, 0xed, 0xfe}, []byte(payload)...)
}

func writeFakeExecutable(t *testing.T) string {
	t.Helper()

	executable := filepath.Join(t.TempDir(), "kobo-gateway")
	require.NoError(
		t,
		os.WriteFile(executable, machO64Header("old"), 0o755),
	)

	return executable
}

func TestSelfUpdate(t *testing.T) {
	downloads := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, kobogateway.DownloadPath, r.URL.Path)
			_, _ = w.Write(machO64Header("new"))
		},
	))
	defer downloads.Close()

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client())

	err := updater.SelfUpdate(context.Background(), downloads.URL)

	require.NoError(t, err)
	data, err := os.ReadFile(executable)
	require.NoError(t, err)
	assert.Equal(t, machO64Header("new"), data)

	info, err := os.Stat(executable)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	// The temp download file is gone after the rename.
	_, err = os.Stat(executable + ".update")
	assert.True(t, os.IsNotExist(err))
}

func TestSelfUpdateRejectsNonBinary(t *testing.T) {
	downloads := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>not a binary</html>"))
		},
	))
	defer downloads.Close()

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client())

	err := updater.SelfUpdate(context.Background(), downloads.URL)

	assert.ErrorContains(t, err, "not a valid gateway binary")

	data, readErr := os.ReadFile(executable)
	require.NoError(t, readErr)
	assert.Equal(t, machO64Header("old"), data)
}

func TestSelfUpdateDownloadError(t *testing.T) {
	downloads := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer downloads.Close()

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client())

	err := updater.SelfUpdate(context.Background(), downloads.URL)

	assert.ErrorContains(t, err, "update download failed")
}

func TestSelfUpdateUnreachableServer(t *testing.T) {
	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, http.DefaultClient)

	err := updater.SelfUpdate(
		context.Background(),
		"http://127.0.0.1:1/nope",
	)

	assert.ErrorContains(t, err, "could not download update")
}

func TestSelfUpdateAcceptsFatBinary(t *testing.T) {
	fat := append([]byte{0xca, 0xfe, 0xba, 0xbe}, []byte("universal")...)
	downloads := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(fat)
		},
	))
	defer downloads.Close()

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client())

	require.NoError(t, updater.SelfUpdate(context.Background(), downloads.URL))
}
