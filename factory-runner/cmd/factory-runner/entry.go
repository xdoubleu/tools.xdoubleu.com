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
	a := &app{
		home:     home,
		execPath: execPath,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		store:    runner.NewKeychain(),
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
		activity: runner.NewActivity(runner.NewCaffeinate()),
		load:     loadLaunchAgent,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := a.run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
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
