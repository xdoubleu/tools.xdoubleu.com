package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
			home:        t.TempDir(),
			execPath:    "/bin/factory-runner",
			stdout:      out,
			stderr:      errs,
			store:       store,
			secrets:     newMemSecrets(),
			stdin:       strings.NewReader(""),
			interactive: false,
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
			control: nil,
			menu:    nil,
			jobs:    nil,
			open:    func(context.Context, string) error { return nil },
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

func TestNoArgsInsideAnAppBundleInstallsAndOpensTheRunnerPage(t *testing.T) {
	e := newEnv(t)
	bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
		"factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
	require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
	e.app.execPath = bundle

	var opened []string
	e.app.open = func(_ context.Context, u string) error {
		opened = append(opened, u)
		return nil
	}

	assert.Equal(t, 0, e.do())
	assert.Equal(t, []string{runner.LaunchAgentPath(e.app.home)}, *e.loaded)
	require.Len(t, opened, 1)
	assert.Equal(t, "https://tools.xdoubleu.com/factory/runners", opened[0])
}

func TestNoArgsInAnAppBundleStillInstallsWhenOpeningThePageFails(t *testing.T) {
	e := newEnv(t)
	bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
		"factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
	require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
	e.app.execPath = bundle

	e.app.open = func(context.Context, string) error { return errors.New("open: nope") }
	assert.Equal(t, 0, e.do())
	assert.Len(t, *e.loaded, 1, "the LaunchAgent is loaded even when the page can't open")
}

func TestNoArgsInAnAppBundleDoesNotDowngradeAnInstalledBinary(t *testing.T) {
	e := newEnv(t)
	bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
		"factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
	require.NoError(t, os.WriteFile(bundle, []byte("stale"), 0o755))
	e.app.execPath = bundle
	installed := runner.InstalledBinaryPath(e.app.home)
	require.NoError(t, os.MkdirAll(filepath.Dir(installed), 0o700))
	require.NoError(t, os.WriteFile(installed, []byte("newer"), 0o755))

	assert.Equal(t, 0, e.do())
	data, err := os.ReadFile(installed)
	require.NoError(t, err)
	assert.Equal(t, "newer", string(data), "a second open keeps the installed binary")
}

func TestNoArgsInAnAppBundleSkipsOpeningThePageWhenConfigIsBroken(t *testing.T) {
	e := newEnv(t)
	bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
		"factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
	require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
	e.app.execPath = bundle

	path := runner.ConfigPath(e.app.home)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))

	var opened []string
	e.app.open = func(_ context.Context, u string) error {
		opened = append(opened, u)
		return nil
	}
	assert.Equal(t, 0, e.do(), "install still succeeds with a broken config")
	assert.Empty(t, opened, "an invalid server never opens a page")
}

func TestNoArgsInAnAppBundleSkipsOpeningThePageWhenTheServerIsUnusable(t *testing.T) {
	e := newEnv(t)
	bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
		"factory-runner")
	require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
	require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
	e.app.execPath = bundle
	// A config whose server can't yield an origin (no scheme) — past the CLI's
	// own validation, which only happens when the user configures it.
	path := runner.ConfigPath(e.app.home)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"server":"not-a-url"}`), 0o600))

	var opened []string
	e.app.open = func(_ context.Context, u string) error {
		opened = append(opened, u)
		return nil
	}
	assert.Equal(t, 0, e.do())
	assert.Empty(t, opened, "a malformed server never opens a page")
}

func TestNoArgsInAnAppBundleReportsInstallFailures(t *testing.T) {
	t.Run("binary copy", func(t *testing.T) {
		e := newEnv(t)
		bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
			"factory-runner")
		require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
		e.app.execPath = bundle // missing file: InstallBinary fails
		assert.Equal(t, 1, e.do())
		assert.Contains(t, e.errs.String(), "factory-runner:")
	})
	t.Run("LaunchAgent write", func(t *testing.T) {
		e := newEnv(t)
		bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
			"factory-runner")
		require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
		require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
		e.app.execPath = bundle
		require.NoError(t, os.WriteFile(filepath.Join(e.app.home, "Library"), nil, 0o600))
		assert.Equal(t, 1, e.do())
	})
	t.Run("launchctl load", func(t *testing.T) {
		e := newEnv(t)
		bundle := filepath.Join(t.TempDir(), "FactoryRunner.app", "Contents", "MacOS",
			"factory-runner")
		require.NoError(t, os.MkdirAll(filepath.Dir(bundle), 0o755))
		require.NoError(t, os.WriteFile(bundle, []byte("b"), 0o755))
		e.app.execPath = bundle
		e.app.load = func(string) error { return errors.New("launchctl failed") }
		assert.Equal(t, 1, e.do())
	})
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

func TestOpenURL(t *testing.T) {
	t.Setenv("PATH", "")
	err := openURL(context.Background(), "https://example.com")
	assert.Error(t, err, "without `open` on PATH openURL reports the failure")
}

func TestTheDaemonRunsBesideTheMenuBarWithTheBoardPage(t *testing.T) {
	e := newEnv(t)
	e.app.control = runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	var got menuInfo
	e.app.menu = func(
		ctx context.Context, info menuInfo, work func(context.Context) error,
	) error {
		got = info
		return work(ctx)
	}
	assert.Equal(t, 0, e.do("run"))
	assert.Equal(t, "https://tools.xdoubleu.com/factory/runners", got.BoardURL)
	assert.Same(t, e.app.control, got.Control)
	assert.Equal(t, []string{"frt_stored"}, *e.ran)
}

func TestPausedFollowsTheControl(t *testing.T) {
	e := newEnv(t)
	assert.False(t, e.app.paused(), "no control, never paused")
	e.app.control = runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	require.NoError(t, e.app.control.SetPaused(true))
	assert.True(t, e.app.paused())
}
