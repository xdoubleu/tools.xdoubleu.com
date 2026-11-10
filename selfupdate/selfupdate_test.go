package selfupdate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/selfupdate"
	"tools.xdoubleu.com/selfupdate/updatesig"
)

func machO(payload string) []byte {
	return append([]byte{0xcf, 0xfa, 0xed, 0xfe}, []byte(payload)...)
}

func serve(t *testing.T, body []byte, seed string) *httptest.Server {
	t.Helper()
	sig, err := updatesig.Sign(seed, body)
	require.NoError(t, err)
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads/rel.txt":
			_, _ = w.Write([]byte("abc\n"))
		case "/downloads/bin":
			_, _ = w.Write(body)
		case "/downloads/bin.sig":
			_, _ = w.Write([]byte(sig))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv
}

func updater(t *testing.T, pub string) (*selfupdate.Updater, string) {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "app")
	require.NoError(t, os.WriteFile(exe, machO("old"), 0o755))
	u := selfupdate.New("test", "bin", "rel.txt", pub, time.Second)
	u.ExecutablePath = func() (string, error) { return exe, nil }
	return u, exe
}

func TestReleaseReadsTheServedRelease(t *testing.T) {
	pub, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	srv := serve(t, machO("new"), seed)
	u, _ := updater(t, pub)

	got, err := u.Release(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "abc", got)
}

func TestInstallReplacesTheExecutableWithASignedBinary(t *testing.T) {
	pub, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	srv := serve(t, machO("new"), seed)
	u, exe := updater(t, pub)

	replaced, err := u.Install(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	assert.True(t, replaced)
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, machO("new"), data)
}

func TestInstallLeavesTheExecutableWhileBusy(t *testing.T) {
	pub, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	srv := serve(t, machO("new"), seed)
	u, exe := updater(t, pub)

	busy := func() bool { return false }
	replaced, err := u.Install(context.Background(), srv.URL, busy)
	require.NoError(t, err)
	assert.False(t, replaced)
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, machO("old"), data)
}

func TestInstallRejectsBadInput(t *testing.T) {
	pub, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	_, otherSeed, err := updatesig.GenerateKey()
	require.NoError(t, err)

	t.Run("no key", func(t *testing.T) {
		u, _ := updater(t, "")
		_, err = u.Install(context.Background(), "http://unused", nil)
		require.ErrorIs(t, err, selfupdate.ErrSigningUnset)
	})
	t.Run("wrong signer", func(t *testing.T) {
		u, _ := updater(t, pub)
		_, err = u.Install(context.Background(), serve(t, machO("new"), otherSeed).URL, nil)
		require.ErrorIs(t, err, updatesig.ErrBadSignature)
	})
	t.Run("not a binary", func(t *testing.T) {
		u, _ := updater(t, pub)
		_, err = u.Install(context.Background(), serve(t, []byte("text"), seed).URL, nil)
		assert.ErrorContains(t, err, "not a valid test binary")
	})
	t.Run("missing download", func(t *testing.T) {
		u, _ := updater(t, pub)
		u.BinaryName = "gone"
		_, err = u.Install(context.Background(), serve(t, machO("new"), seed).URL, nil)
		assert.ErrorContains(t, err, "update download failed")
	})
}
