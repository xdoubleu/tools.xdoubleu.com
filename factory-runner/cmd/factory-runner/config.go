package main

import (
	"errors"
	"fmt"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

var errConfigUsage = errors.New(
	"usage: factory-runner config show | server <url> | " +
		"repos add|remove <owner/name> | repos list",
)

func (a *app) config(args []string) error {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return err
	}
	switch {
	case len(args) == 1 && args[0] == "show":
		fmt.Fprintf(a.stdout, "server: %s\nrepos: %v\n", cfg.Server, cfg.Repos)
		return nil
	case len(args) == 2 && args[0] == "server":
		if err = cfg.SetServer(args[1]); err != nil {
			return err
		}
	case len(args) >= 2 && args[0] == "repos":
		if err = a.configRepos(&cfg, args[1:]); err != nil {
			return err
		}
		if args[1] == "list" {
			return nil
		}
	default:
		return errConfigUsage
	}
	return cfg.Save(path)
}

func (a *app) configRepos(cfg *runner.Config, args []string) error {
	switch {
	case len(args) == 2 && args[0] == "add":
		return cfg.AddRepo(args[1])
	case len(args) == 2 && args[0] == "remove":
		if !cfg.RemoveRepo(args[1]) {
			return fmt.Errorf("%s is not in the allowlist", args[1])
		}
		return nil
	case len(args) == 1 && args[0] == "list":
		return a.listRepos()
	default:
		return errConfigUsage
	}
}

func (a *app) listRepos() error {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return err
	}
	for _, r := range cfg.Repos {
		fmt.Fprintln(a.stdout, r)
	}
	return nil
}
