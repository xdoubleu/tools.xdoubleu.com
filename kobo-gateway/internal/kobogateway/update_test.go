package kobogateway_test

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
	"tools.xdoubleu.com/kobo-gateway/internal/updatesig"
)

// machO64Header is the magic prefix of a darwin/arm64 binary.
func machO64Header(payload string) []byte {
	return append([]byte{0xcf, 0xfa, 0xed, 0xfe}, []byte(payload)...)
}

// testKey signs the fake downloads served by signedDownloads.
type testKey struct {
	pub  ed25519.PublicKey
	seed string
}

func newTestKey(t *testing.T) testKey {
	t.Helper()

	pubB64, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	pub, err := updatesig.ParsePublicKey(pubB64)
	require.NoError(t, err)

	return testKey{pub: pub, seed: seed}
}

// signedDownloads serves body at DownloadPath and its signature (by signer)
// at SignaturePath.
func signedDownloads(t *testing.T, signer testKey, body []byte) *httptest.Server {
	t.Helper()

	sig, err := updatesig.Sign(signer.seed, body)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case kobogateway.DownloadPath:
				_, _ = w.Write(body)
			case kobogateway.SignaturePath:
				_, _ = w.Write([]byte(sig))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		},
	))
	t.Cleanup(server.Close)

	return server
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
	key := newTestKey(t)
	downloads := signedDownloads(t, key, machO64Header("new"))

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)

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
	key := newTestKey(t)
	downloads := signedDownloads(t, key, []byte("<html>not a binary</html>"))

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)

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
	updater := kobogateway.NewUpdaterFor(
		executable, downloads.Client(), newTestKey(t).pub,
	)

	err := updater.SelfUpdate(context.Background(), downloads.URL)

	assert.ErrorContains(t, err, "update download failed")
}

func TestSelfUpdateRejectsBadSignatures(t *testing.T) {
	key := newTestKey(t)
	cases := map[string]*httptest.Server{
		"signed by another key": signedDownloads(t, newTestKey(t), machO64Header("evil")),
		"signature missing": httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == kobogateway.DownloadPath {
					_, _ = w.Write(machO64Header("evil"))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
		)),
	}

	for name, downloads := range cases {
		t.Run(name, func(t *testing.T) {
			defer downloads.Close()

			executable := writeFakeExecutable(t)
			updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)

			require.Error(t, updater.SelfUpdate(context.Background(), downloads.URL))

			data, err := os.ReadFile(executable)
			require.NoError(t, err)
			assert.Equal(t, machO64Header("old"), data, "binary left untouched")
		})
	}
}

func TestSelfUpdateDisabledWithoutKey(t *testing.T) {
	downloads := signedDownloads(t, newTestKey(t), machO64Header("new"))
	executable := writeFakeExecutable(t)

	for _, updater := range []*kobogateway.Updater{
		kobogateway.NewUpdaterFor(executable, downloads.Client(), nil),
		kobogateway.NewUpdater(""),
		kobogateway.NewUpdater("not-a-key"),
	} {
		err := updater.SelfUpdate(context.Background(), downloads.URL)
		require.ErrorIs(t, err, kobogateway.ErrUpdateSigningUnset)
	}
}

func TestSelfUpdateUnreachableServer(t *testing.T) {
	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, http.DefaultClient, newTestKey(t).pub)

	err := updater.SelfUpdate(
		context.Background(),
		"http://127.0.0.1:1/nope",
	)

	assert.ErrorContains(t, err, "could not download update")
}

func TestSelfUpdateSkipsResignOutsideBundle(t *testing.T) {
	// Not inside a ".app", so resignBundle is a no-op and codesign isn't needed.
	key := newTestKey(t)
	downloads := signedDownloads(t, key, machO64Header("new"))

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)

	require.NoError(t, updater.SelfUpdate(context.Background(), downloads.URL))
}

func TestSelfUpdateFailsWhenResignFails(t *testing.T) {
	if _, err := exec.LookPath("codesign"); err != nil {
		t.Skip("codesign not available")
	}

	// No Info.plist: codesign rejects the bundle, exercising the error path.
	appDir := filepath.Join(t.TempDir(), "KoboGateway.app")
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macOSDir, 0o755))
	executable := filepath.Join(macOSDir, "kobo-gateway")
	require.NoError(t, os.WriteFile(executable, machO64Header("old"), 0o755))

	key := newTestKey(t)
	downloads := signedDownloads(t, key, machO64Header("new"))

	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)
	err := updater.SelfUpdate(context.Background(), downloads.URL)

	assert.ErrorContains(t, err, "could not re-sign updated app bundle")
}

func TestSelfUpdateResignsAppBundle(t *testing.T) {
	if _, err := exec.LookPath("codesign"); err != nil {
		t.Skip("codesign not available")
	}

	appDir := filepath.Join(t.TempDir(), "KoboGateway.app")
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macOSDir, 0o755))
	executable := filepath.Join(macOSDir, "kobo-gateway")
	require.NoError(t, os.WriteFile(executable, machO64Header("old"), 0o755))

	// codesign needs a minimal Info.plist to recognize the bundle.
	infoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleIdentifier</key><string>com.example.test</string>
	<key>CFBundleExecutable</key><string>kobo-gateway</string>
</dict></plist>`
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "Contents", "Info.plist"),
		[]byte(infoPlist),
		0o644,
	))

	key := newTestKey(t)
	downloads := signedDownloads(t, key, machO64Header("new"))

	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)
	require.NoError(t, updater.SelfUpdate(context.Background(), downloads.URL))

	verify := exec.Command("codesign", "--verify", "--strict", appDir)
	out, err := verify.CombinedOutput()
	assert.NoError(t, err, string(out))
}

func TestAppBundlePath(t *testing.T) {
	rawExecutable := writeFakeExecutable(t)
	assert.Equal(t, "", kobogateway.AppBundlePath(rawExecutable))

	appDir := filepath.Join(t.TempDir(), "KoboGateway.app")
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macOSDir, 0o755))
	bundledExecutable := filepath.Join(macOSDir, "kobo-gateway")
	assert.Equal(t, appDir, kobogateway.AppBundlePath(bundledExecutable))
}

func TestSelfUpdateAcceptsFatBinary(t *testing.T) {
	fat := append([]byte{0xca, 0xfe, 0xba, 0xbe}, []byte("universal")...)
	key := newTestKey(t)
	downloads := signedDownloads(t, key, fat)

	executable := writeFakeExecutable(t)
	updater := kobogateway.NewUpdaterFor(executable, downloads.Client(), key.pub)

	require.NoError(t, updater.SelfUpdate(context.Background(), downloads.URL))
}
