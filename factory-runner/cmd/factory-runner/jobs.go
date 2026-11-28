package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/wardcase/session"
	"tools.xdoubleu.com/wardcase/vm"
)

// diskFree reads a volume's free bytes; tests replace it.
var diskFree = runner.FreeBytes //nolint:gochecknoglobals // tests replace it

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
		Image:      cfg.ReleaseImage(WardcaseVersion),
		Exec:       exec,
		Proxy:      nil,
		ProxyPort:  runner.EgressProxyPort,
		GoModCache: cfg.GoModCache,
		NPMCache:   cfg.NPMCache,
		// The store wardcase's own sessions share, so a flake's tools are fetched once.
		NixStore:    filepath.Join(a.home, ".local", "state", "wardcase", "nix"),
		NixStoreMax: 0,
		CPUs:        "",
		Memory:      "",
		TempDir:     "",
	}
	if err := box.Check(ctx); err != nil {
		return nil, fmt.Errorf("jobs cannot run on this Mac: %w", err)
	}
	if err := os.MkdirAll(box.NixStore, dataDirPerm); err != nil {
		return nil, fmt.Errorf("nix store directory: %w", err)
	}
	dataDir := cfg.DataPath(a.home)
	if err := os.MkdirAll(dataDir, dataDirPerm); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}
	jobs := &runner.JobRunner{
		NoCIGrace: 0,
		OpenChat:  nil,
		API:       runner.NewClient(cfg.Server, Release),
		Token:     token,
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
		Preflight: runner.Preflights(
			runner.FilePressurePreflight(runner.SystemFiles),
			runner.NixStorePreflight(box.NixStore),
		),
		StopVMs:       box.StopRuns,
		Paused:        a.holdClaims,
		MaxJobs:       a.maxJobs,
		Mode:          a.mode,
		Harness:       runner.ClaudeCode{},
		ClaudeToken:   storedSecret(a.secrets, runner.SecretClaude),
		OpenRouterKey: storedSecret(a.secrets, runner.SecretOpenRouter),
		AuthorEmail:   cfg.BotEmail,
		MCP:           session.JobMCPServers,
		Sessions:      runner.NewSessionAPI,
		OnLimit:       a.onLimit,
		Proxy: func(
			job *runner.ClaimedJob, servers []runner.MCPServer,
		) (runner.ProxyListener, error) {
			return newJobProxy(cfg, job, a.secrets, servers, log)
		},
		Activity:         a.activity,
		DataDir:          dataDir,
		DiskFree:         func() (uint64, error) { return diskFree(dataDir) },
		Secrets:          []string{token},
		GitHub:           runner.NewGitHubClient(),
		ServerURL:        cfg.Server,
		PublishRetry:     0,
		CheckInEvery:     0,
		Minute:           0,
		EventsEvery:      0,
		PollEvery:        0,
		RunningPollEvery: 0,
		MaxWatch:         0,
		Sleep:            nil,
		Now:              nil,
		Log:              log,
	}
	// Before a pull, which needs room, and on every retry of a failed one: a
	// runner whose jobs are off still sweeps.
	if a.control != nil {
		a.control.SetClaimSources(runner.ClaimSources{
			Max: jobs.MaxJobsNow, Jobs: jobs.RunningJobs, LimitUntil: jobs.LimitUntil,
			Fault: jobs.Fault,
		})
	}
	jobs.MakeRoom(ctx, func(ctx context.Context) { pruneImages(ctx, cfg, exec, log) })
	if err := ensureImage(ctx, cfg, exec); err != nil {
		return nil, err
	}
	pruneImages(ctx, cfg, exec, log)
	return jobs, nil
}

// onLimit is what a usage limit does and the model it falls back to, read each
// time so `config on-limit` takes effect without a restart.
func (a *app) onLimit() (string, string) {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return runner.OnLimitWait, ""
	}
	return cfg.EffectiveOnLimit(), cfg.FallbackModel
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

// maxJobs is how many jobs run at once, read each time so the menu bar's choice
// takes effect without a restart.
func (a *app) maxJobs() int {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return 1
	}
	return cfg.EffectiveMaxJobs()
}

// setMaxJobsTo is the menu bar's choice, saved to the config.
func (a *app) setMaxJobsTo(n int) error {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return err
	}
	if err = cfg.SetMaxJobs(n); err != nil {
		return err
	}
	return cfg.Save(path)
}

// notClaiming is whether the claim loop stays idle: Stop, or no harness chosen.
func (a *app) notClaiming() bool {
	return a.mode() == runner.ModeOff
}

// holdClaims is notClaiming, or an update pending: a newer release waiting for
// running jobs, or an installed one waiting to restart.
func (a *app) holdClaims() bool {
	return a.notClaiming() ||
		(a.control != nil && (a.control.Draining() || a.control.Restarting()))
}

// pruneImages drops base images of other versions once this one is in place. It
// runs at start, so also after every self-update.
func pruneImages(
	ctx context.Context, cfg runner.Config, exec runner.Execer, log *slog.Logger,
) {
	if !cfg.JobImageAutoDownload(WardcaseVersion) {
		return
	}
	runner.PruneJobImages(ctx, exec, cfg.ReleaseImage(WardcaseVersion), log)
}

// ensureImage pulls the wardcase base image of this build's version when the
// container CLI lacks it. An explicit `config image <ref>` or a dev build skips
// the pull, so a missing image then surfaces as a `make image` error when a job
// runs.
func ensureImage(ctx context.Context, cfg runner.Config, exec runner.Execer) error {
	if !cfg.JobImageAutoDownload(WardcaseVersion) {
		return nil
	}
	image := cfg.ReleaseImage(WardcaseVersion)
	if err := vm.EnsureImage(ctx, exec, image, io.Discard); err != nil {
		return fmt.Errorf("job image %s: %w", image, err)
	}
	return nil
}

// storedSecret reads a stored credential afresh on each call.
func storedSecret(secrets runner.SecretStore, name string) func() (string, error) {
	return func() (string, error) { return secrets.GetSecret(name) }
}
