package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/updatesig"
)

// dataDirPerm keeps job data to this user.
const dataDirPerm = 0o700

// newJobRunner assembles the claim loop: the container sandbox (run through exec),
// the host's git and the server's job API. It fails, naming what to fix, when this
// Mac cannot run jobs.
func newJobRunner(
	ctx context.Context,
	a *app,
	cfg runner.Config,
	token string,
	log *slog.Logger,
	exec runner.Execer,
) (*runner.JobRunner, error) {
	box := &runner.ContainerSandbox{
		Image:      cfg.ReleaseImage(Release),
		Exec:       exec,
		Proxy:      nil,
		ProxyPort:  runner.EgressProxyPort,
		GoModCache: cfg.GoModCache,
		NPMCache:   cfg.NPMCache,
		CPUs:       "",
		Memory:     "",
		TempDir:    "",
	}
	if err := box.Check(ctx); err != nil {
		return nil, fmt.Errorf("jobs cannot run on this Mac: %w", err)
	}
	if err := ensureImage(ctx, cfg, exec); err != nil {
		return nil, err
	}
	dataDir := cfg.DataPath(a.home)
	if err := os.MkdirAll(dataDir, dataDirPerm); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}
	return &runner.JobRunner{
		API:   runner.NewClient(cfg.Server, Release),
		Token: token,
		// Read each time, so `config repos add` takes effect without a restart.
		Allowed: func() ([]string, error) {
			current, _, err := a.loadConfig()
			return current.Repos, err
		},
		Git: &runner.Git{
			Dir:        dataDir,
			RemoteURL:  runner.GitHubURL,
			Run:        nil,
			RunWithEnv: nil,
		},
		Sandbox: box,
		Paused:  a.notClaiming,
		Mode:    a.mode,
		Harness: runner.ClaudeCode{},
		ClaudeToken: func() (string, error) {
			return a.secrets.GetSecret(runner.SecretClaude)
		},
		AuthorEmail: cfg.BotEmail,
		Proxy: func(job *runner.ClaimedJob) (runner.ProxyListener, error) {
			return newJobProxy(cfg, job, a.secrets, log)
		},
		Activity:     a.activity,
		DataDir:      dataDir,
		Secrets:      []string{token},
		GitHub:       runner.NewGitHubClient(),
		ServerURL:    cfg.Server,
		PublishRetry: 0,
		CheckInEvery: 0,
		Minute:       0,
		EventsEvery:  0,
		Log:          log,
	}, nil
}

// paused is whether the owner stopped this runner from the menu bar.
func (a *app) paused() bool {
	return a.control != nil && a.control.Paused()
}

// mode is this Mac's mode, read each time so `config mode` takes effect without a
// restart. A read error counts as off.
func (a *app) mode() string {
	cfg, _, err := a.loadConfig()
	if err != nil || a.paused() {
		return runner.ModeOff
	}
	return cfg.EffectiveMode()
}

// notClaiming is whether the claim loop stays idle: Stop, or no harness chosen.
func (a *app) notClaiming() bool {
	return a.mode() == runner.ModeOff
}

// ensureImage makes sure the versioned job image this release needs is present,
// fetching it from the release server (with the update key) when it is not. An
// explicit `config image <ref>` or a dev build skips the fetch and just uses the
// reference, so a missing image then surfaces as a `make image` error when a job
// runs.
func ensureImage(ctx context.Context, cfg runner.Config, exec runner.Execer) error {
	if !cfg.JobImageAutoDownload(Release) {
		return nil
	}
	origin, err := runner.Origin(cfg.Server)
	if err != nil {
		return fmt.Errorf("job image: %w", err)
	}
	if UpdatePublicKey == "" {
		// No trusted key to verify a download, as with self-update: rely on a
		// locally built image, and let a missing one surface when a job's VM
		// starts rather than blocking every job.
		return nil
	}
	publicKey, err := updatesig.ParsePublicKey(UpdatePublicKey)
	if err != nil {
		return fmt.Errorf("job image: %w", err)
	}
	ensurer := &runner.ImageEnsurer{
		Client:    &http.Client{Timeout: runner.ImageFetchTimeout},
		Origin:    origin,
		Release:   Release,
		Image:     cfg.ReleaseImage(Release),
		PublicKey: publicKey,
		Exec:      exec,
		TempDir:   "",
	}
	if err = ensurer.Ensure(ctx); err != nil {
		return fmt.Errorf("job image %s: %w", ensurer.Image, err)
	}
	return nil
}
