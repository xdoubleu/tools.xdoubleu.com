package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"tools.xdoubleu.com/factory-runner/internal/runner"
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
		Image:      cfg.JobImage(),
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
		Harness: runner.ClaudeCode{},
		ClaudeToken: func() (string, error) {
			return a.secrets.GetSecret(runner.SecretClaude)
		},
		AuthorEmail: cfg.AuthorEmail(),
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
