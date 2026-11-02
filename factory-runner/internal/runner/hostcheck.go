package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrHostCheck marks a finished job whose work the host won't publish; its
// message says why and becomes the job's outcome.
var ErrHostCheck = errors.New("host check failed")

const (
	// minSecretLen skips values too short to search for without false hits.
	minSecretLen  = 8
	workflowsPath = ".github/workflows"
	shortHashLen  = 12
)

// Inspect checks the work an agent left on branch before anything is pushed: it
// has commits beyond base, none touches .github/workflows/, and no secret value
// is in any commit or message. The clone belongs to the agent, so nothing runs
// in it: the branch is fetched into the host's mirror (objects verified) as a
// ref named for the job, and every check reads that copy. The ref stays for the
// push; Discard removes it.
func (g *Git) Inspect(
	ctx context.Context,
	owner, name, base, branch, jobID, cloneDir string,
	secrets []string,
) error {
	if err := plainGitDir(cloneDir); err != nil {
		return err
	}
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")
	ref := jobRef(jobID)

	err := g.mustGit(ctx, mirror,
		"-c", "core.hooksPath=/dev/null", "-c", "transfer.fsckObjects=true",
		"fetch", "--no-tags", "--no-write-fetch-head", "--",
		cloneDir, "+refs/heads/"+branch+":"+ref)
	if err != nil {
		return fmt.Errorf("%w: could not read the branch %s from the clone: %w",
			ErrHostCheck, branch, err)
	}

	span := "refs/heads/" + base + ".." + ref
	if err = g.hasCommits(ctx, mirror, span); err != nil {
		return err
	}
	if err = g.noWorkflowChanges(ctx, mirror, "refs/heads/"+base+"..."+ref); err != nil {
		return err
	}
	return g.noSecrets(ctx, mirror, span, secrets)
}

// Discard removes the ref Inspect made for the job.
func (g *Git) Discard(ctx context.Context, owner, name, jobID string) {
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")
	_, _ = g.git(ctx, mirror, "update-ref", "-d", jobRef(jobID))
}

func jobRef(jobID string) string { return "refs/factory/" + jobID }

// plainGitDir refuses a clone whose .git the agent turned into a pointer (a
// gitfile or symlink to some other repo on the host) or gave borrowed objects.
func plainGitDir(cloneDir string) error {
	dotGit := filepath.Join(cloneDir, ".git")
	info, err := os.Lstat(dotGit)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: the clone's .git is not a plain directory", ErrHostCheck)
	}
	alternates := filepath.Join(dotGit, "objects", "info", "alternates")
	if _, err = os.Lstat(alternates); err == nil {
		return fmt.Errorf("%w: the clone borrows objects from elsewhere", ErrHostCheck)
	}
	return nil
}

func (g *Git) hasCommits(ctx context.Context, mirror, span string) error {
	out, err := g.git(ctx, mirror, "rev-list", "--count", span)
	if err != nil {
		return fmt.Errorf("%w: could not count commits: %w", ErrHostCheck, err)
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil || n == 0 {
		return fmt.Errorf("%w: the agent made no commits on the branch", ErrHostCheck)
	}
	return nil
}

func (g *Git) noWorkflowChanges(ctx context.Context, mirror, span string) error {
	out, err := g.git(ctx, mirror, "diff", "--name-only", "-z", "--no-renames",
		"--no-ext-diff", span)
	if err != nil {
		return fmt.Errorf("%w: could not list changed files: %w", ErrHostCheck, err)
	}
	for _, path := range bytes.Split(out, []byte{0}) {
		// macOS is case-insensitive: .GitHub/Workflows is the same directory.
		p := strings.ToLower(string(path))
		if p == workflowsPath || strings.HasPrefix(p, workflowsPath+"/") {
			return fmt.Errorf("%w: the agent changed %s, which it may not touch",
				ErrHostCheck, string(path))
		}
	}
	return nil
}

// noSecrets looks for each secret in every commit's changes and message, so a
// value committed and removed again is still found. The value is never printed.
func (g *Git) noSecrets(
	ctx context.Context,
	mirror, span string,
	secrets []string,
) error {
	for _, secret := range secrets {
		if len(secret) < minSecretLen {
			continue
		}
		for _, search := range [][]string{
			{"-S" + secret},
			{"--fixed-strings", "--grep=" + secret},
		} {
			args := append([]string{"log", "--no-ext-diff", "--no-textconv",
				"--format=%H"}, search...)
			out, err := g.git(ctx, mirror, append(args, span)...)
			if err != nil {
				return fmt.Errorf("%w: could not search the commits: %w", ErrHostCheck, err)
			}
			if hash := strings.TrimSpace(string(out)); hash != "" {
				commit := firstLine(hash)
				return fmt.Errorf("%w: a credential value appears in commit %s",
					ErrHostCheck, commit[:min(len(commit), shortHashLen)])
			}
		}
	}
	return nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
