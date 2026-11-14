package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

const (
	gitDirPerm  = 0o700
	gitFilePerm = 0o600
)

// mirrorLocks has one mutex per mirror: jobs on one repo, run side by side, must
// not update the mirror while another clones from it.
//
//nolint:gochecknoglobals // the mirrors on disk are shared by every Git
var mirrorLocks sync.Map

// lockMirror takes the mirror's lock, in-process and across runner processes
// (a flock on <mirror>.lock), and returns its release.
func lockMirror(mirror string) (func(), error) {
	mu, _ := mirrorLocks.LoadOrStore(mirror, &sync.Mutex{})
	m, _ := mu.(*sync.Mutex)
	m.Lock()
	if err := os.MkdirAll(filepath.Dir(mirror), gitDirPerm); err != nil {
		m.Unlock()
		return nil, fmt.Errorf("create mirror dir: %w", err)
	}
	f, err := os.OpenFile(mirror+".lock", os.O_CREATE|os.O_RDWR, gitFilePerm)
	if err != nil {
		m.Unlock()
		return nil, fmt.Errorf("open mirror lock: %w", err)
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		m.Unlock()
		return nil, fmt.Errorf("lock mirror: %w", err)
	}
	return func() {
		_ = f.Close() // closing drops the flock
		m.Unlock()
	}, nil
}

// cloneFromMirror clones the mirror to dest on branch. A mirror whose refs got
// ahead of its objects updates fine but can't be cloned from, so a failure
// removes the mirror and the partial dest, re-clones the mirror and retries once.
func (g *Git) cloneFromMirror(
	ctx context.Context,
	remote, mirror, branch, dest string,
) error {
	args := []string{"clone", "--no-hardlinks", "--branch", branch, "--", mirror, dest}
	if g.mustGit(ctx, "", args...) == nil {
		return nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("remove a partial clone: %w", err)
	}
	if err := os.RemoveAll(mirror); err != nil {
		return fmt.Errorf("remove a broken mirror: %w", err)
	}
	if err := g.mustGit(ctx, "", "clone", "--mirror", "--", remote, mirror); err != nil {
		return err
	}
	return g.mustGit(ctx, "", args...)
}

// Git prepares a job's clone on the host before the sandbox starts. After that
// the clone belongs to the agent: it can write .git/hooks and .git/config, which
// the host's git would run. So the host never runs git inside a job clone
// without `-c core.hooksPath=/dev/null` and a clean config.
type Git struct {
	// Dir holds the mirrors: <Dir>/mirrors/<owner>/<name>.git.
	Dir string
	// RemoteURL is where a repo lives; the host's own credentials fetch from it.
	RemoteURL func(owner, name string) string
	// Run executes git with args in dir (empty: the current directory) and returns
	// its combined output. Nil runs the real git.
	Run func(ctx context.Context, dir string, args ...string) ([]byte, error)
	// RunWithEnv, when set, runs the git that needs extra environment (a push's
	// token), so a test can look at both.
	RunWithEnv func(
		ctx context.Context, dir string, env []string, args ...string,
	) ([]byte, error)
}

// GitHubURL is the usual remote.
func GitHubURL(owner, name string) string {
	return "https://github.com/" + owner + "/" + name + ".git"
}

func (g *Git) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if g.Run != nil {
		return g.Run(ctx, dir, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.Bytes(), err
}

func (g *Git) mustGit(ctx context.Context, dir string, args ...string) error {
	if out, err := g.git(ctx, dir, args...); err != nil {
		return fmt.Errorf("git %s: %w: %s", args[0], err, bytes.TrimSpace(out))
	}
	return nil
}

// updateMirror brings the mirror up to date, cloning it when it is missing. A
// mirror that can't be updated (an interrupted first clone leaves one that never
// works) is removed and cloned again once.
func (g *Git) updateMirror(ctx context.Context, remote, mirror string) error {
	if _, err := os.Stat(mirror); err == nil {
		if g.mustGit(ctx, mirror, "remote", "update", "--prune") == nil {
			return nil
		}
		if err = os.RemoveAll(mirror); err != nil {
			return fmt.Errorf("remove a broken mirror: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(mirror), gitDirPerm); err != nil {
		return fmt.Errorf("create mirror dir: %w", err)
	}
	return g.mustGit(ctx, "", "clone", "--mirror", "--", remote, mirror)
}

// Restore makes a standalone clone of owner/name at dest on the existing branch,
// as a Reply's job continues it, and returns the commit the branch is at. The
// branch comes from the mirror, so it must have been pushed.
func (g *Git) Restore(
	ctx context.Context,
	owner, name, branch, dest string,
) (string, error) {
	remote := g.RemoteURL(owner, name)
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")
	unlock, err := lockMirror(mirror)
	if err != nil {
		return "", err
	}
	defer unlock()
	if err = g.updateMirror(ctx, remote, mirror); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(dest), gitDirPerm); err != nil {
		return "", fmt.Errorf("create job dir: %w", err)
	}
	if err = g.cloneFromMirror(ctx, remote, mirror, branch, dest); err != nil {
		return "", err
	}
	if err = g.mustGit(ctx, dest, "remote", "set-url", "origin", remote); err != nil {
		return "", err
	}
	out, err := g.git(ctx, dest, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w: %s", err, bytes.TrimSpace(out))
	}
	return string(bytes.TrimSpace(out)), nil
}

// Clone makes a standalone clone of owner/name at dest, on a new branch off base.
// The repo is mirrored on the host first (fetched, never exposed to the sandbox)
// and the clone copies from the mirror without hard links, so a write through the
// mount can't reach the mirror's objects. A git worktree won't do: its .git points
// at the host's git directory.
func (g *Git) Clone(ctx context.Context, owner, name, base, branch, dest string) error {
	remote := g.RemoteURL(owner, name)
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")
	unlock, err := lockMirror(mirror)
	if err != nil {
		return err
	}
	defer unlock()

	if err = g.updateMirror(ctx, remote, mirror); err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(dest), gitDirPerm); err != nil {
		return fmt.Errorf("create job dir: %w", err)
	}
	if err = g.cloneFromMirror(ctx, remote, mirror, base, dest); err != nil {
		return err
	}
	// The clone points at the real remote, not the mirror's path.
	if err = g.mustGit(ctx, dest, "remote", "set-url", "origin", remote); err != nil {
		return err
	}
	return g.mustGit(ctx, dest, "checkout", "-b", branch)
}
