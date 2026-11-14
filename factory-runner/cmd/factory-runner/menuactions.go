package main

import (
	"context"
	"fmt"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// chooseMode is the menu's Mode choice: the config's mode, saved. It also sets the
// stop flag the way `config mode` does, so a running runner follows without a
// restart.
func (a *app) chooseMode(mode string) error {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return err
	}
	if err = a.setMode(&cfg, mode); err != nil {
		return err
	}
	return cfg.Save(path)
}

// configMode is the mode in the config, which the menu checkmarks; mode() also
// reads the stop flag.
func (a *app) configMode() string {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return runner.ModeOff
	}
	return cfg.EffectiveMode()
}

// repos is the allowlist.
func (a *app) repos() []string {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return nil
	}
	return cfg.Repos
}

// addRepo allows a repo; the claim loop reads the allowlist per job.
func (a *app) addRepo(repo string) error {
	return a.editRepos(func(cfg *runner.Config) error { return cfg.AddRepo(repo) })
}

// removeRepo drops a repo from the allowlist.
func (a *app) removeRepo(repo string) error {
	return a.editRepos(func(cfg *runner.Config) error {
		if !cfg.RemoveRepo(repo) {
			return fmt.Errorf("%s is not in the allowlist", repo)
		}
		return nil
	})
}

func (a *app) editRepos(edit func(*runner.Config) error) error {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return err
	}
	if err = edit(&cfg); err != nil {
		return err
	}
	return cfg.Save(path)
}

// serverURL is the API root pairing talks to.
func (a *app) serverURL() string {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return runner.DefaultServer
	}
	return cfg.Server
}

// pairWith exchanges a pairing code for a device token and keeps it. A non-empty
// server first replaces the configured one. A Mac with no mode yet is set to off,
// so only the owner's choice in the menu starts it claiming.
func (a *app) pairWith(ctx context.Context, code, server string) (string, error) {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return "", err
	}
	if server != "" && server != cfg.Server {
		if err = cfg.SetServer(server); err != nil {
			return "", err
		}
	}
	token, name, err := a.newBeat(cfg.Server).Pair(ctx, code)
	if err != nil {
		return "", err
	}
	if err = a.store.Set(token); err != nil {
		return "", err
	}
	if cfg.Mode == "" {
		if err = cfg.SetMode(runner.ModeOff); err != nil {
			return "", err
		}
	}
	if err = cfg.Save(path); err != nil {
		return "", err
	}
	if a.control != nil {
		a.control.MarkPaired()
	}
	return name, nil
}

// awaitPairing blocks, with only the menu to pair from, until a token is stored.
func (a *app) awaitPairing(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("waiting to be paired: %w", ctx.Err())
	case <-a.control.AwaitPaired():
		return a.store.Get()
	}
}
