package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

var errConfigUsage = errors.New(
	"usage: factory-runner config show | mode off|claude | server <url> | " +
		"repos add|remove <owner/name> | repos list | image <ref> | " +
		"data|gocache|npmcache <dir> | bot-email <address> | secret claude|openrouter",
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
		fmt.Fprintf(a.stdout, "gocache: %s\nnpmcache: %s\nbot-email: %s\nmode: %s\n",
			orNone(cfg.GoModCache), orNone(cfg.NPMCache), cfg.AuthorEmail(), cfg.EffectiveMode())
		return nil
	case verb == "mode" && len(rest) == 1:
		err = a.setMode(&cfg, rest[0])
	case verb == "server" && len(rest) == 1:
		err = cfg.SetServer(rest[0])
	case verb == "image" && len(rest) == 1:
		err = cfg.SetImage(rest[0])
	case verb == "bot-email" && len(rest) == 1:
		err = cfg.SetBotEmail(rest[0])
	case verb == "secret" && len(rest) == 1:
		return a.storeSecret(rest[0])
	case slices.Contains([]string{"data", "gocache", "npmcache"}, verb) &&
		len(rest) == 1:
		err = a.setDir(&cfg, verb, rest[0])
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

// maxSecretBytes bounds what is read from stdin as a secret.
const maxSecretBytes = 4096

// storeSecret reads a credential from stdin, never from argv (which other processes
// can read) or the shell history, and keeps it in the keychain.
func (a *app) storeSecret(name string) error {
	if a.interactive {
		fmt.Fprintf(a.stderr, "Reading the %s secret from the terminal until Ctrl-D "+
			"(it is echoed). Prefer: pbpaste | factory-runner config secret %s\n",
			name, name)
	}
	raw, err := io.ReadAll(io.LimitReader(a.stdin, maxSecretBytes+1))
	if err != nil {
		return fmt.Errorf("read the %s secret: %w", name, err)
	}
	if len(raw) > maxSecretBytes {
		return fmt.Errorf("the %s secret is longer than %d bytes", name, maxSecretBytes)
	}
	if err = a.secrets.SetSecret(name, strings.TrimSpace(string(raw))); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "Stored the %s secret in the keychain.\n", name)
	return nil
}

// orNone says what an unset cache means: the VM has no registry access, so a job
// that needs dependencies fails without one.
func orNone(dir string) string {
	if dir == "" {
		return "none (jobs get no dependencies from the registry)"
	}
	return dir
}

// setDir sets a directory; the caches must exist already, as the VM mounts them.
func (a *app) setDir(cfg *runner.Config, which, dir string) error {
	if err := cfg.SetDir(which, dir); err != nil {
		return err
	}
	if which == "data" {
		return nil
	}
	info, err := os.Stat(dir) //nolint:gosec // the operator's own directory
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory that exists", dir)
	}
	return nil
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

// setMode chooses the harness. Off is the menu bar's Stop: both write the same
// paused flag, so a running runner follows without a restart.
func (a *app) setMode(cfg *runner.Config, mode string) error {
	if err := cfg.SetMode(mode); err != nil {
		return err
	}
	if a.control == nil {
		return nil
	}
	return a.control.SetPaused(cfg.EffectiveMode() == runner.ModeOff)
}
