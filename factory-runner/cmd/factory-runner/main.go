// Command factory-runner is the Mac-side helper of the software factory: it
// pairs with the board and keeps its heartbeat so the board shows it online.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// Release and UpdatePublicKey are set at build time. An empty key disables
// self-update.
var (
	Release         = runner.DevRelease //nolint:gochecknoglobals // stamped by -ldflags
	UpdatePublicKey = ""                //nolint:gochecknoglobals // stamped by -ldflags
)

// exitUsage is the exit status for a command line that makes no sense.
const exitUsage = 2

const usage = `usage:
  factory-runner pair <code>          pair this Mac with the code shown on the board
  factory-runner run                  heartbeat until stopped (run by the LaunchAgent)
  factory-runner install              install and load the LaunchAgent
  factory-runner config show
  factory-runner config server <url>
  factory-runner config repos add|remove <owner/name>
  factory-runner config repos list
  factory-runner version
`

// app holds everything a command touches, so tests can swap it.
type app struct {
	home     string
	execPath string
	stdout   io.Writer
	stderr   io.Writer
	store    runner.TokenStore
	newBeat  func(server string) pairHeartbeater
	daemon   daemonFunc
	updater  func(server string) (runner.SelfUpdater, error)
	activity *runner.Activity
	load     func(plistPath string) error
}

type daemonFunc func(
	ctx context.Context,
	hb runner.Heartbeater,
	token string,
	opts runner.Options,
) error

// pairHeartbeater is the server side the commands need.
type pairHeartbeater interface {
	runner.Heartbeater
	Pair(ctx context.Context, code string) (token, name string, err error)
}

func (a *app) run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.stderr, usage)
		return exitUsage
	}
	var err error
	switch args[0] {
	case "pair":
		err = a.pair(ctx, args[1:])
	case "run":
		err = a.runDaemon(ctx)
	case "install":
		err = a.install()
	case "config":
		err = a.config(args[1:])
	case "version":
		fmt.Fprintln(a.stdout, Release)
	default:
		fmt.Fprint(a.stderr, usage)
		return exitUsage
	}
	if err != nil {
		fmt.Fprintln(a.stderr, "factory-runner:", err)
		return 1
	}
	return 0
}

func (a *app) loadConfig() (runner.Config, string, error) {
	path := runner.ConfigPath(a.home)
	cfg, err := runner.LoadConfig(path)
	return cfg, path, err
}

func (a *app) pair(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: factory-runner pair <code>")
	}
	cfg, _, err := a.loadConfig()
	if err != nil {
		return err
	}
	token, name, err := a.newBeat(cfg.Server).Pair(ctx, args[0])
	if err != nil {
		return err
	}
	if err = a.store.Set(token); err != nil {
		return err
	}
	fmt.Fprintf(
		a.stdout,
		"Paired as %q. Run `factory-runner install` to keep it online.\n",
		name,
	)
	return nil
}

func (a *app) runDaemon(ctx context.Context) error {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return err
	}
	token, err := a.store.Get()
	if err != nil {
		return err
	}
	upd, err := a.updater(cfg.Server)
	if err != nil {
		return err
	}
	opts := runner.Options{Updater: upd, Activity: a.activity}
	err = a.daemon(ctx, a.newBeat(cfg.Server), token, opts)
	if errors.Is(err, runner.ErrRestart) {
		fmt.Fprintln(a.stdout, "Updated to a new release; restarting.")
		return nil
	}
	return err
}

func (a *app) install() error {
	installed, err := runner.InstallBinary(a.home, a.execPath)
	if err != nil {
		return err
	}
	path, err := runner.InstallLaunchAgent(a.home, installed)
	if err != nil {
		return err
	}
	if err = a.load(path); err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "Installed and loaded", path)
	return nil
}
