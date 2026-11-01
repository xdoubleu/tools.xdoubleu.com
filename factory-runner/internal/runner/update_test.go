package runner_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/factory-runner/internal/updatesig"
)

func always() bool { return true }

func machO(payload string) []byte {
	return append([]byte{0xcf, 0xfa, 0xed, 0xfe}, []byte(payload)...)
}

type signer struct {
	pubB64 string
	pub    ed25519.PublicKey
	seed   string
}

func newSigner(t *testing.T) signer {
	t.Helper()
	pubB64, seed, err := updatesig.GenerateKey()
	require.NoError(t, err)
	pub, err := updatesig.ParsePublicKey(pubB64)
	require.NoError(t, err)
	return signer{pubB64: pubB64, pub: pub, seed: seed}
}

func newRelease(version string, body []byte, by signer, noSig bool) release {
	return release{version: version, body: body, sigBy: by, noSig: noSig}
}

type release struct {
	version string
	body    []byte
	sigBy   signer
	noSig   bool
}

func serveRelease(t *testing.T, r release) *httptest.Server {
	t.Helper()
	sig, err := updatesig.Sign(r.sigBy.seed, r.body)
	require.NoError(t, err)
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
			switch q.URL.Path {
			case "/downloads/" + runner.ReleaseFile:
				_, _ = w.Write([]byte(r.version + "\n"))
			case "/downloads/" + runner.BinaryName:
				_, _ = w.Write(r.body)
			case "/downloads/" + runner.BinaryName + ".sig":
				if r.noSig {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = w.Write([]byte(sig))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	)
	t.Cleanup(srv.Close)
	return srv
}

func fakeExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(path, machO("old"), 0o755))
	return path
}

func newUpdater(srv *httptest.Server, current, key, exe string) *runner.Updater {
	u := runner.NewUpdater(srv.URL, current, key)
	u.SetExecutable(exe)
	return u
}

func TestUpdateInstallsANewerSignedRelease(t *testing.T) {
	key := newSigner(t)
	srv := serveRelease(t, newRelease("new", machO("new"), key, false))
	exe := fakeExecutable(t)

	updated, err := newUpdater(
		srv,
		"old",
		key.pubB64,
		exe,
	).Update(context.Background(), always)
	require.NoError(t, err)
	assert.True(t, updated)

	got, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, machO("new"), got)
	info, err := os.Stat(exe)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	assert.NoFileExists(t, exe+".update")
}

func TestUpdateDoesNothingWhenCurrent(t *testing.T) {
	key := newSigner(t)
	srv := serveRelease(t, newRelease("same", machO("x"), key, false))
	exe := fakeExecutable(t)
	updated, err := newUpdater(
		srv,
		"same",
		key.pubB64,
		exe,
	).Update(context.Background(), always)
	require.NoError(t, err)
	assert.False(t, updated)
	got, _ := os.ReadFile(exe)
	assert.Equal(t, machO("old"), got)
}

func TestUpdateIsDisabledWithoutAKeyOrOnADevBuild(t *testing.T) {
	key := newSigner(t)
	srv := serveRelease(t, newRelease("new", machO("new"), key, false))
	exe := fakeExecutable(t)

	for _, badKey := range []string{"", "not-base64!"} {
		_, err := newUpdater(srv, "old", badKey, exe).Update(context.Background(), always)
		require.ErrorIs(t, err, runner.ErrUpdateSigningUnset)
	}
	for _, current := range []string{runner.DevRelease, ""} {
		updated, err := newUpdater(
			srv,
			current,
			key.pubB64,
			exe,
		).Update(context.Background(), always)
		require.NoError(t, err)
		assert.False(t, updated)
	}
}

func TestUpdateRefusesWhatItCannotTrust(t *testing.T) {
	key, other := newSigner(t), newSigner(t)
	exe := fakeExecutable(t)
	for name, r := range map[string]release{
		"signed by another key": newRelease("new", machO("evil"), other, false),
		"not a binary":          newRelease("new", []byte("#!/bin/sh\nevil"), key, false),
		"missing signature":     newRelease("new", machO("new"), key, true),
	} {
		srv := serveRelease(t, r)
		updated, err := newUpdater(
			srv,
			"old",
			key.pubB64,
			exe,
		).Update(context.Background(), always)
		require.Error(t, err, name)
		assert.False(t, updated, name)
		got, _ := os.ReadFile(exe)
		assert.Equal(t, machO("old"), got, name+": the binary is untouched")
	}
}

func TestUpdateFailures(t *testing.T) {
	key := newSigner(t)
	ctx := context.Background()

	missing := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(missing.Close)
	_, err := newUpdater(missing, "old", key.pubB64, fakeExecutable(t)).Update(ctx, always)
	require.Error(t, err, "no release file")

	srv := serveRelease(t, newRelease("new", machO("new"), key, false))
	u := newUpdater(srv, "old", key.pubB64, "")
	u.SetExecutableError(errors.New("no executable"))
	_, err = u.Update(ctx, always)
	require.Error(t, err)

	// The executable's directory can't be written.
	_, err = newUpdater(srv, "old", key.pubB64,
		filepath.Join(t.TempDir(), "gone", "factory-runner")).Update(ctx, always)
	require.Error(t, err)

	// Renaming over a directory fails and leaves no temp file behind.
	dir := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "child"), 0o755))
	_, err = newUpdater(srv, "old", key.pubB64, dir).Update(ctx, always)
	require.Error(t, err)
	assert.NoFileExists(t, dir+".update")

	down := runner.NewUpdater("http://127.0.0.1:1", "old", key.pubB64)
	_, err = down.Update(ctx, always)
	require.Error(t, err)

	bad := runner.NewUpdater("http://bad host", "old", key.pubB64)
	_, err = bad.Update(ctx, always)
	require.Error(t, err)
}

func TestOrigin(t *testing.T) {
	got, err := runner.Origin("https://tools.xdoubleu.com/api")
	require.NoError(t, err)
	assert.Equal(t, "https://tools.xdoubleu.com", got)

	got, err = runner.Origin("http://localhost:8000/api")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8000", got)

	for _, bad := range []string{"", "tools.xdoubleu.com", "/api"} {
		_, err = runner.Origin(bad)
		require.Error(t, err, bad)
	}
}

func TestUpdateWaitsIfAJobStartedDuringTheDownload(t *testing.T) {
	key := newSigner(t)
	srv := serveRelease(t, newRelease("new", machO("new"), key, false))
	exe := fakeExecutable(t)

	updated, err := newUpdater(srv, "old", key.pubB64, exe).
		Update(context.Background(), func() bool { return false })
	require.NoError(t, err)
	assert.False(t, updated)
	got, _ := os.ReadFile(exe)
	assert.Equal(t, machO("old"), got, "the binary is untouched")
	assert.NoFileExists(t, exe+".update")
}

func TestADevBuildNeverUpdatesEvenWithoutAKey(t *testing.T) {
	srv := serveRelease(t, newRelease("new", machO("new"), newSigner(t), false))
	updated, err := newUpdater(srv, runner.DevRelease, "", fakeExecutable(t)).
		Update(context.Background(), always)
	require.NoError(t, err)
	assert.False(t, updated)
}
