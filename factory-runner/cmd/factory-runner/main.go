// Command factory-runner is the Mac-side helper of the software factory: it
// pairs with the board, keeps its heartbeat so the board shows it online, and runs
// the jobs it claims, each in its own VM.
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
  factory-runner run                  heartbeat and run jobs (run by the LaunchAgent)
  factory-runner install              install and load the LaunchAgent
  factory-runner config show
  factory-runner config server <url>
  factory-runner config repos add|remove <owner/name>
  factory-runner config repos list
  factory-runner config image <ref>
  factory-runner config data|gocache|npmcache <dir>
  factory-runner config bot-email <address>
  factory-runner config secret claude|openrouter   (the value is read from stdin)
  factory-runner version
`

// app holds everything a command touches, so tests can swap it.
type app struct {
	home     string
	execPath string
	stdout   io.Writer
	stderr   io.Writer
	store    runner.TokenStore
	secrets  runner.SecretStore
	stdin    io.Reader
	// interactive is whether stdin is a terminal, where a secret would be echoed.
	interactive bool
	newBeat     func(server string) pairHeartbeater
	daemon      daemonFunc
	updater     func(server string) (runner.SelfUpdater, error)
	activity    *runner.Activity
	// control is the pause flag and status the menu bar shows; nil in tests.
	control *runner.Control
	// menu runs work beside the menu bar icon; nil runs it bare.
	menu menuFunc
	load func(plistPath string) error
	// open launches a URL in the user's browser, for the app install's pairing step.
	open func(ctx context.Context, url string) error
	// jobs builds the claim loop; nil runs the heartbeat alone.
	jobs jobsFunc
}

// jobLoop claims and runs jobs until its context ends.
type jobLoop interface {
	Loop(ctx context.Context) error
}

// jobsFunc builds the job loop for a config and device token, and fails if jobs
// cannot run on this Mac (no container CLI, say).
type jobsFunc func(
	ctx context.Context,
	cfg runner.Config,
	token string,
) (jobLoop, error)

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
		// Double-clicking the installed app runs the binary with no args; that
		// is the drag-to-Applications install, not a bare binary usage error.
		if runner.AppBundlePath(a.execPath) != "" {
			return a.installFromApp(ctx)
		}
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
	opts := runner.Options{Updater: upd, Activity: a.activity, Control: a.control}
	beat := func(ctx context.Context) error {
		return a.daemon(ctx, a.newBeat(cfg.Server), token, opts)
	}
	return a.withMenu(ctx, func(ctx context.Context) error {
		return a.serve(ctx, cfg, token, beat)
	})
}

// withMenu runs work beside the menu bar icon when there is one.
func (a *app) withMenu(
	ctx context.Context,
	work func(context.Context) error,
) error {
	if a.menu == nil || a.control == nil {
		return work(ctx)
	}
	return a.menu(ctx, menuInfo{
		Control:  a.control,
		Release:  Release,
		Home:     a.home,
		ExecPath: a.execPath,
	}, work)
}

func (a *app) serve(
	ctx context.Context,
	cfg runner.Config,
	token string,
	beat func(context.Context) error,
) error {
	var err error
	var loop jobLoop
	if a.jobs != nil {
		if loop, err = a.jobs(ctx, cfg, token); err != nil {
			// The heartbeat and self-update still run: a newer release may be what
			// fixes this, and the board should show the runner is alive.
			fmt.Fprintln(a.stderr, "factory-runner: jobs are off:", err)
			loop = nil
		}
	}
	if loop == nil {
		err = beat(ctx)
	} else {
		err = alongside(ctx, beat, loop.Loop, a.activity.Idle, idlePoll)
	}
	if errors.Is(err, runner.ErrRestart) {
		fmt.Fprintln(a.stdout, "Updated to a new release; restarting.")
		return nil
	}
	return err
}

// idlePoll is how often a restart checks whether the last job has finished.
const idlePoll = time.Second

// alongside runs two loops until either ends, then stops the other. A revoked
// token ends both; so does a restart after an update, but only once no job runs,
// as stopping the loop would kill it. The error returned is the most telling one
// (revoked, then restart), not whichever loop happened to finish first.
func alongside(
	ctx context.Context,
	first, second func(context.Context) error,
	idle func() bool,
	poll time.Duration,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2) //nolint:mnd // one result per loop
	go func() { errs <- first(ctx) }()
	go func() { errs <- second(ctx) }()
	one := <-errs
	if errors.Is(one, runner.ErrRestart) {
		waitIdle(ctx, idle, poll)
	}
	cancel()
	return mostTelling(one, <-errs)
}

// waitIdle returns once idle reports true, or ctx ends.
func waitIdle(ctx context.Context, idle func() bool, poll time.Duration) {
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for !idle() {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func mostTelling(a, b error) error {
	for _, target := range []error{runner.ErrRevoked, runner.ErrRestart} {
		if errors.Is(a, target) {
			return a
		}
		if errors.Is(b, target) {
			return b
		}
	}
	return cmp.Or(a, b)
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

// installFromApp runs `install` as a double-clicked .app, then opens the board's
// Runner page so the user can pair. Opening the page is best-effort: installing
// and loading the LaunchAgent is the point, and a windowless app can't tell the
// user anything else. A second open after a self-update leaves the newer
// installed binary in place: only the first open copies the bundle's binary,
// while the plist is rewritten and reloaded every time.
func (a *app) installFromApp(ctx context.Context) int {
	installed := runner.InstalledBinaryPath(a.home)
	if _, err := os.Stat(installed); errors.Is(err, os.ErrNotExist) {
		installed, err = runner.InstallBinary(a.home, a.execPath)
		if err != nil {
			fmt.Fprintln(a.stderr, "factory-runner:", err)
			return 1
		}
	}
	if path, err := runner.InstallLaunchAgent(a.home, installed); err != nil {
		fmt.Fprintln(a.stderr, "factory-runner:", err)
		return 1
	} else if err = a.load(path); err != nil {
		fmt.Fprintln(a.stderr, "factory-runner:", err)
		return 1
	}
	cfg, _, err := a.loadConfig()
	if err != nil {
		return 0
	}
	root, err := runner.Origin(cfg.Server)
	if err != nil {
		return 0
	}
	if err = a.open(ctx, root+"/factory/runners"); err != nil {
		fmt.Fprintln(a.stderr, "factory-runner: open the runner page:", err)
	}
	return 0
}
