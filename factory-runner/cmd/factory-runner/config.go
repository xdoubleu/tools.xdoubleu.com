package main

import (
	"errors"
	"fmt"
	"slices"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

var errConfigUsage = errors.New(
	"usage: factory-runner config show | server <url> | " +
		"repos add|remove <owner/name> | repos list | image <ref> | " +
		"data|gocache|npmcache <dir>",
)

func (a *app) config(args []string) error {
	if len(args) == 0 {
		return errConfigUsage
	}
	cfg, path, err := a.loadConfig()
	if err != nil {
		return err
	}
	verb, rest := args[0], args[1:]
	switch {
	case verb == "show" && len(rest) == 0:
		fmt.Fprintf(a.stdout, "server: %s\nrepos: %v\nimage: %s\ndata: %s\n",
			cfg.Server, cfg.Repos, cfg.JobImage(), cfg.DataPath(a.home))
		fmt.Fprintf(a.stdout, "gocache: %s\nnpmcache: %s\n", cfg.GoModCache, cfg.NPMCache)
		return nil
	case verb == "server" && len(rest) == 1:
		err = cfg.SetServer(rest[0])
	case verb == "image" && len(rest) == 1:
		err = cfg.SetImage(rest[0])
	case slices.Contains([]string{"data", "gocache", "npmcache"}, verb) &&
		len(rest) == 1:
		err = cfg.SetDir(verb, rest[0])
	case verb == "repos" && len(rest) > 0:
		if rest[0] == "list" && len(rest) == 1 {
			return a.listRepos()
		}
		err = a.configRepos(&cfg, rest)
	default:
		return errConfigUsage
	}
	if err != nil {
		return err
	}
	return cfg.Save(path)
}

func (a *app) configRepos(cfg *runner.Config, args []string) error {
	if len(args) != 2 { //nolint:mnd // a verb and a repo
		return errConfigUsage
	}
	switch args[0] {
	case "add":
		return cfg.AddRepo(args[1])
	case "remove":
		if !cfg.RemoveRepo(args[1]) {
			return fmt.Errorf("%s is not in the allowlist", args[1])
		}
		return nil
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
