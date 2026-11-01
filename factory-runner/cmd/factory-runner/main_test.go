package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type memStore struct {
	token  string
	getErr error
	setErr error
}

func (m *memStore) Get() (string, error) { return m.token, m.getErr }
func (m *memStore) Set(t string) error {
	m.token = t
	return m.setErr
}

type fakeServer struct {
	pairErr error
	servers []string
}

func (f *fakeServer) Heartbeat(context.Context, string) error { return nil }
func (f *fakeServer) Pair(_ context.Context, code string) (string, string, error) {
	if f.pairErr != nil {
		return "", "", f.pairErr
	}
	return "frt_" + code, "Home Mac", nil
}

type env struct {
	app    *app
	out    *bytes.Buffer
	errs   *bytes.Buffer
	store  *memStore
	server *fakeServer
	ran    *[]string
	loaded *[]string
}

func newEnv(t *testing.T) env {
	t.Helper()
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	store, server := &memStore{
		token:  "frt_stored",
		getErr: nil,
		setErr: nil,
	}, &fakeServer{pairErr: nil, servers: nil}
	var ran, loaded []string
	return env{
		app: &app{
			home:     t.TempDir(),
			execPath: "/bin/factory-runner",
			stdout:   out,
			stderr:   errs,
			store:    store,
			newBeat: func(s string) pairHeartbeater {
				server.servers = append(server.servers, s)
				return server
			},
			daemon: func(
				_ context.Context, _ runner.Heartbeater, token string, _ runner.Options,
			) error {
				ran = append(ran, token)
				return nil
			},
			updater:  func(string) (runner.SelfUpdater, error) { return nil, nil },
			activity: runner.NewActivity(nil),
			load: func(p string) error {
				loaded = append(loaded, p)
				return nil
			},
		},
		out: out, errs: errs, store: store, server: server, ran: &ran, loaded: &loaded,
	}
}

func (e env) do(args ...string) int {
	return e.app.run(context.Background(), args)
}

func TestNoOrUnknownCommandPrintsUsage(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 2, e.do())
	assert.Contains(t, e.errs.String(), "usage:")
	assert.Equal(t, 2, e.do("nope"))
}

func TestVersion(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 0, e.do("version"))
	assert.Equal(t, Release+"\n", e.out.String())
}

func TestPairStoresTheToken(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 0, e.do("pair", "ABCDE-FGHJK"))
	assert.Equal(t, "frt_ABCDE-FGHJK", e.store.token)
	assert.Contains(t, e.out.String(), "Home Mac")
	assert.Equal(t, []string{runner.DefaultServer}, e.server.servers)
}

func TestPairFailures(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 1, e.do("pair"), "no code")
	assert.Equal(t, 1, e.do("pair", "a", "b"), "two codes")

	e.server.pairErr = errors.New("invalid or expired pairing code")
	assert.Equal(t, 1, e.do("pair", "x"))
	assert.Contains(t, e.errs.String(), "invalid or expired")
	assert.Equal(t, "frt_stored", e.store.token, "the old token stays")

	e.server.pairErr = nil
	e.store.setErr = errors.New("keychain locked")
	assert.Equal(t, 1, e.do("pair", "x"))
}

func TestBrokenConfigStopsEveryCommand(t *testing.T) {
	e := newEnv(t)
	path := runner.ConfigPath(e.app.home)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
	for _, args := range [][]string{
		{"pair", "x"}, {"run"}, {"config", "show"}, {"config", "repos", "list"},
	} {
		assert.Equal(t, 1, e.do(args...), args)
	}
}

func TestRunHeartbeatsWithTheStoredToken(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 0, e.do("run"))
	assert.Equal(t, []string{"frt_stored"}, *e.ran)
}

func TestRunNeedsAToken(t *testing.T) {
	e := newEnv(t)
	e.store.getErr = runner.ErrNoToken
	assert.Equal(t, 1, e.do("run"))
	assert.Contains(t, e.errs.String(), "not paired")
	assert.Empty(t, *e.ran)
}

func TestRunReportsARevokedToken(t *testing.T) {
	e := newEnv(t)
	e.app.daemon = func(
		context.Context, runner.Heartbeater, string, runner.Options,
	) error {
		return runner.ErrRevoked
	}
	assert.Equal(t, 1, e.do("run"))
	assert.Contains(t, e.errs.String(), "revoked")
}

func TestInstallWritesAndLoadsThePlist(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("b"), 0o755))
	e.app.execPath = src
	assert.Equal(t, 0, e.do("install"))
	assert.Equal(t, []string{runner.LaunchAgentPath(e.app.home)}, *e.loaded)
	data, err := os.ReadFile(runner.LaunchAgentPath(e.app.home))
	require.NoError(t, err)
	assert.Contains(t, string(data), "factory-runner")

	e.app.load = func(string) error { return errors.New("launchctl failed") }
	assert.Equal(t, 1, e.do("install"))

	blocked := newEnv(t)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(blocked.app.home, "Library"), nil, 0o600),
	)
	assert.Equal(t, 1, blocked.do("install"))
}

func TestConfigReposAddListRemove(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 0, e.do("config", "repos", "add", "xdoubleu/tools.xdoubleu.com"))
	assert.Equal(t, 0, e.do("config", "repos", "add", "a/b"))
	assert.Equal(t, 0, e.do("config", "repos", "list"))
	assert.Equal(t, "a/b\nxdoubleu/tools.xdoubleu.com\n", e.out.String())

	assert.Equal(t, 0, e.do("config", "repos", "remove", "A/B"))
	assert.Equal(t, 1, e.do("config", "repos", "remove", "a/b"), "already gone")
	assert.Equal(t, 1, e.do("config", "repos", "add", "not-a-repo"))
	assert.Equal(t, 1, e.do("config", "repos", "frobnicate", "x"))
}

func TestConfigServerAndShow(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 0, e.do("config", "server", "https://example.com/api/"))
	assert.Equal(t, 1, e.do("config", "server", "http://example.com"))

	e.out.Reset()
	assert.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "server: https://example.com/api\n")

	assert.Equal(t, 0, e.do("pair", "x"))
	assert.Equal(t, "https://example.com/api", e.server.servers[0])
}

func TestConfigUsageErrors(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{
		{"config"}, {"config", "show", "x"}, {"config", "server"}, {"config", "repos"},
		{"config", "wat"},
	} {
		assert.Equal(t, 1, e.do(args...), args)
	}
}

func TestConfigSaveFailure(t *testing.T) {
	e := newEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(e.app.home, "Library"), nil, 0o600))
	assert.Equal(t, 1, e.do("config", "repos", "add", "a/b"))
}

func TestRunRestartsAfterASelfUpdate(t *testing.T) {
	e := newEnv(t)
	e.app.daemon = func(
		context.Context, runner.Heartbeater, string, runner.Options,
	) error {
		return runner.ErrRestart
	}
	assert.Equal(t, 0, e.do("run"), "launchd restarts the new binary")
	assert.Contains(t, e.out.String(), "restarting")
}

func TestRunPassesTheUpdaterAndActivityToTheDaemon(t *testing.T) {
	e := newEnv(t)
	var got runner.Options
	e.app.daemon = func(
		_ context.Context, _ runner.Heartbeater, _ string, opts runner.Options,
	) error {
		got = opts
		return nil
	}
	assert.Equal(t, 0, e.do("run"))
	assert.Same(t, e.app.activity, got.Activity)
}

func TestRunFailsWhenTheUpdaterCannotBeBuilt(t *testing.T) {
	e := newEnv(t)
	e.app.updater = func(string) (runner.SelfUpdater, error) {
		return nil, errors.New("invalid server URL")
	}
	assert.Equal(t, 1, e.do("run"))
	assert.Empty(t, *e.ran)
}

func TestInstallCopiesTheBinaryAndRunsTheAgentFromThere(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(t.TempDir(), "factory-runner")
	require.NoError(t, os.WriteFile(src, []byte("binary"), 0o755))
	e.app.execPath = src

	assert.Equal(t, 0, e.do("install"))
	installed := runner.InstalledBinaryPath(e.app.home)
	assert.FileExists(t, installed)
	plist, err := os.ReadFile(runner.LaunchAgentPath(e.app.home))
	require.NoError(t, err)
	assert.Contains(t, string(plist), installed)
}

func TestInstallFailsWithoutAnExecutableToCopy(t *testing.T) {
	e := newEnv(t)
	e.app.execPath = filepath.Join(t.TempDir(), "missing")
	assert.Equal(t, 1, e.do("install"))
}
