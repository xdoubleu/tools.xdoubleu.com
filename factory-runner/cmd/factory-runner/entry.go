package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/appbundle"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no home directory:", err)
		os.Exit(1)
	}
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no executable path:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	keychain := runner.NewKeychain()
	a := &app{
		home:        home,
		execPath:    execPath,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		store:       keychain,
		secrets:     keychain,
		stdin:       os.Stdin,
		interactive: isTerminal(os.Stdin),
		newBeat: func(server string) pairHeartbeater {
			return runner.NewClient(server, Release)
		},
		daemon: func(
			ctx context.Context,
			hb runner.Heartbeater,
			token string,
			opts runner.Options,
		) error {
			return runner.Run(ctx, hb, token, log, opts)
		},
		updater: func(server string) (runner.SelfUpdater, error) {
			origin, originErr := runner.Origin(server)
			if originErr != nil {
				return nil, originErr
			}
			return runner.NewUpdater(origin, Release, UpdatePublicKey), nil
		},
		activity:         nil,
		control:          nil,
		menu:             runMenu,
		load:             loadLaunchAgent,
		reload:           reloadLaunchAgentDetached,
		open:             openURL,
		jobs:             nil,
		installedRelease: runner.BinaryRelease,
		sign:             appbundle.Resign,
	}
	a.activity = runner.NewActivity(runner.NewCaffeinate())
	a.control = runner.NewControl(runner.PausePath(home), Release, a.activity)
	a.control.SetModeSource(a.mode)
	a.jobs = func(ctx context.Context, cfg runner.Config, token string) (jobLoop, error) {
		return newJobRunner(ctx, a, cfg, token, log, runner.OSExecer{BinDirs: nil})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := a.run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

// isTerminal reports whether f is a terminal rather than a pipe or file.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// openURL opens u in the user's default browser.
func openURL(ctx context.Context, u string) error {
	return exec.CommandContext(ctx, "open", u).Run()
}

// reloadLaunchAgentDetached reloads the agent from its own session, after this
// process has had a moment to exit, so the bootout doesn't kill the reload.
func reloadLaunchAgentDetached(plistPath string) error {
	domain := fmt.Sprintf("gui/%d", os.Getuid())

	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c",
		`sleep 2; launchctl bootout "$1" "$2"; launchctl bootstrap "$1" "$2"`,
		"sh", domain, plistPath)
	//nolint:exhaustruct // only Setsid matters
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start launchctl reload: %w", err)
	}
	return cmd.Process.Release()
}

func loadLaunchAgent(plistPath string) error {
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	// Reload: an earlier copy may be loaded; its absence is not an error.
	ctx := context.Background()
	_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, plistPath).Run()
	out, err := exec.CommandContext(ctx, "launchctl", "bootstrap", domain, plistPath).
		CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, out)
	}
	return nil
}
