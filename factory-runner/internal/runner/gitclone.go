package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const gitDirPerm = 0o700

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

// Clone makes a standalone clone of owner/name at dest, on a new branch off base.
// The repo is mirrored on the host first (fetched, never exposed to the sandbox)
// and the clone copies from the mirror without hard links, so a write through the
// mount can't reach the mirror's objects. A git worktree won't do: its .git points
// at the host's git directory.
func (g *Git) Clone(ctx context.Context, owner, name, base, branch, dest string) error {
	remote := g.RemoteURL(owner, name)
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")

	if err := g.updateMirror(ctx, remote, mirror); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), gitDirPerm); err != nil {
		return fmt.Errorf("create job dir: %w", err)
	}
	err := g.mustGit(
		ctx,
		"",
		"clone",
		"--no-hardlinks",
		"--branch",
		base,
		"--",
		mirror,
		dest,
	)
	if err != nil {
		return err
	}
	// The clone points at the real remote, not the mirror's path.
	if err = g.mustGit(ctx, dest, "remote", "set-url", "origin", remote); err != nil {
		return err
	}
	return g.mustGit(ctx, dest, "checkout", "-b", branch)
}
