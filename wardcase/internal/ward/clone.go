package ward

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	reposDir = "repos"
	cacheDir = "repo"
	filePerm = 0o600
)

// safeGit turns off what a repo could make the host's git run: the agent owns the
// cache clone's .git, so its hooks and config are not trusted.
func safeGit() []string {
	return []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}
}

// GitEnvFunc is GitFunc with extra environment, for the git that needs a token.
type GitEnvFunc func(
	ctx context.Context, dir string, env []string, args ...string,
) (string, error)

// HostGit runs the host's git in dir.
func HostGit(ctx context.Context, dir string, args ...string) (string, error) {
	return HostGitEnv(ctx, dir, nil, args...)
}

// HostGitEnv runs the host's git in dir with env added, and never prompts.
func HostGitEnv(
	ctx context.Context,
	dir string,
	env []string,
	args ...string,
) (string, error) {
	//nolint:gosec // git with wardcase's own fixed arguments
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", firstNonFlag(args), err,
			strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// firstNonFlag is the git subcommand, skipping -c options.
func firstNonFlag(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// HostGH runs gh when it is installed; without it every call fails.
func HostGH(ctx context.Context, dir string, args ...string) (string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh: %w", err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err = cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// Source is the repository and branches one session works from.
type Source struct {
	// URL is where the repository is cloned and fetched from.
	URL string
	// Base is the branch the new one is cut from.
	Base string
	// Branch is the new branch the session commits on.
	Branch string
	// Token authenticates over HTTPS; empty uses the host git's own credentials
	// (the `gh` login). It goes to the host's git only, never into the VM.
	Token string
}

// repoLocks has one mutex per cached clone: sessions on one repo, side by side,
// must not fetch into it while another adds a worktree.
//
//nolint:gochecknoglobals // the clones on disk are shared by every Runner
var repoLocks sync.Map

// lockRepo takes the clone's lock, in-process and across processes (a flock on
// <clone>.lock), and returns its release.
func lockRepo(cache string) (func(), error) {
	mu, _ := repoLocks.LoadOrStore(cache, &sync.Mutex{})
	m, _ := mu.(*sync.Mutex)
	m.Lock()
	if err := os.MkdirAll(filepath.Dir(cache), stateDirPerm); err != nil {
		m.Unlock()
		return nil, fmt.Errorf("create the repo directory: %w", err)
	}
	f, err := os.OpenFile(cache+".lock", os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		m.Unlock()
		return nil, fmt.Errorf("open the repo lock: %w", err)
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		m.Unlock()
		return nil, fmt.Errorf("lock the repo: %w", err)
	}
	return func() {
		_ = f.Close() // closing drops the flock
		m.Unlock()
	}, nil
}

// cachePath is where the clone of url lives.
func (r *Runner) cachePath(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(r.StateDir, reposDir, hex.EncodeToString(sum[:])[:checkoutKey],
		cacheDir)
}

// network runs a git command that talks to the remote, with the token as an
// Authorization header in the environment and not in any argument.
func (r *Runner) network(
	ctx context.Context,
	src Source,
	dir string,
	args ...string,
) error {
	var env []string
	if src.Token != "" {
		basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + src.Token))
		env = []string{
			"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader",
			"GIT_CONFIG_VALUE_0=Authorization: Basic " + basic,
		}
		if r.GitEnv == nil {
			return errors.New("a token needs Runner.GitEnv")
		}
	}
	args = append(safeGit(), args...)
	var err error
	if r.GitEnv != nil {
		_, err = r.GitEnv(ctx, dir, env, args...)
	} else {
		_, err = r.Git(ctx, dir, args...)
	}
	return err
}

// refresh brings the cached clone up to date from src.URL, cloning it when it is
// missing. The fetch names the URL, not the clone's origin, so a config the agent
// rewrote cannot send the token elsewhere. A clone that can't be fetched is
// removed and cloned again once.
func (r *Runner) refresh(ctx context.Context, src Source, cache string) error {
	fetch := []string{
		"fetch", "--prune", "--no-tags", "--no-recurse-submodules", "--", src.URL,
		"+refs/heads/*:refs/remotes/origin/*",
	}
	if _, err := os.Stat(cache); err == nil {
		if r.network(ctx, src, cache, fetch...) == nil {
			return nil
		}
		if err = os.RemoveAll(cache); err != nil {
			return fmt.Errorf("remove a broken clone: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(cache), stateDirPerm); err != nil {
		return fmt.Errorf("create the repo directory: %w", err)
	}
	if err := r.network(ctx, src, filepath.Dir(cache),
		"clone", "--no-checkout", "--", src.URL, cache); err != nil {
		_ = os.RemoveAll(cache)
		return fmt.Errorf("clone %s: %w", src.URL, err)
	}
	return nil
}

// checkout gives a session a new worktree on src.Branch, cut from the fresh
// src.Base of the cached clone of src.URL.
func (r *Runner) checkout(ctx context.Context, src Source) (session, error) {
	if src.URL == "" || src.Base == "" || src.Branch == "" {
		return none(), errors.New("a session needs a repository URL, base and branch")
	}
	cache := r.cachePath(src.URL)
	unlock, err := lockRepo(cache)
	if err != nil {
		return none(), err
	}
	defer unlock()
	if err = r.refresh(ctx, src, cache); err != nil {
		return none(), err
	}
	start := "origin/" + src.Base
	if _, err = r.Git(ctx, cache, "rev-parse", "--verify", "--quiet",
		start+"^{commit}"); err != nil {
		return none(), fmt.Errorf("%s has no branch %s", src.URL, src.Base)
	}
	_, _ = r.Git(ctx, cache, "worktree", "prune")
	return r.addWorktree(ctx, cache, src.Branch, start, randomID())
}
