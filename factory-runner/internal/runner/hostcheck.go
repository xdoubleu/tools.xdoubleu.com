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
// has commits that change something, none touches .github/workflows/, and no
// secret value is in any commit, message or author. The clone belongs to the
// agent, so nothing runs in it: the branch is fetched, objects verified, into a
// throwaway repo of the host's (so a rejected commit never reaches the mirror
// later jobs clone from) and every check reads that copy.
func (g *Git) Inspect(
	ctx context.Context,
	owner, name, base, branch, jobID, cloneDir string,
	secrets []string,
) error {
	if err := plainGitDir(cloneDir); err != nil {
		return err
	}
	mirror := filepath.Join(g.Dir, "mirrors", owner, name+".git")
	review := filepath.Join(g.Dir, "reviews", jobID+".git")
	if err := os.RemoveAll(review); err != nil {
		return fmt.Errorf("clear the review repo: %w", err)
	}
	defer os.RemoveAll(review)

	if err := os.MkdirAll(filepath.Dir(review), gitDirPerm); err != nil {
		return fmt.Errorf("create the review dir: %w", err)
	}
	err := g.mustGit(ctx, "", "clone", "--bare", "--no-hardlinks", "--", mirror, review)
	if err != nil {
		return err
	}
	err = g.mustGit(ctx, review,
		"-c", "core.hooksPath=/dev/null", "-c", "transfer.fsckObjects=true",
		"fetch", "--no-tags", "--no-write-fetch-head", "--",
		cloneDir, "+refs/heads/"+branch+":refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("%w: could not read the branch %s from the clone: %w",
			ErrHostCheck, branch, err)
	}

	from, to := "refs/heads/"+base, "refs/heads/"+branch
	span := from + ".." + to
	if err = g.hasChanges(ctx, review, span, from+"..."+to); err != nil {
		return err
	}
	if err = g.noWorkflowChanges(ctx, review, span); err != nil {
		return err
	}
	return g.noSecrets(ctx, review, span, secrets)
}

// plainGitDir refuses a clone whose .git (or the parts git reads objects and
// refs from) the agent turned into a pointer to another place on the host, or
// gave borrowed objects.
func plainGitDir(cloneDir string) error {
	dotGit := filepath.Join(cloneDir, ".git")
	for _, dir := range []string{
		dotGit,
		filepath.Join(dotGit, "objects"),
		filepath.Join(dotGit, "objects", "info"),
		filepath.Join(dotGit, "refs"),
	} {
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
			return fmt.Errorf("%w: the clone's .git is not a plain directory", ErrHostCheck)
		}
	}
	alternates := filepath.Join(dotGit, "objects", "info", "alternates")
	if _, err := os.Lstat(alternates); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: the clone borrows objects from elsewhere", ErrHostCheck)
	}
	return nil
}

// hasChanges refuses a branch with no commits, or whose commits add up to no
// change from base (an empty commit, or a change and its revert).
func (g *Git) hasChanges(ctx context.Context, review, span, net string) error {
	out, err := g.git(ctx, review, "rev-list", "--count", span)
	if err != nil {
		return fmt.Errorf("%w: could not count commits: %w", ErrHostCheck, err)
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil || n == 0 {
		return fmt.Errorf("%w: the agent made no commits on the branch", ErrHostCheck)
	}
	out, err = g.git(ctx, review, "diff", "--name-only", "--no-ext-diff", net)
	if err != nil {
		return fmt.Errorf("%w: could not list changed files: %w", ErrHostCheck, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("%w: the agent's commits change nothing", ErrHostCheck)
	}
	return nil
}

// noWorkflowChanges looks at every commit, merges included, so a workflow change
// that a later commit reverts still counts: it would be in the pushed history.
func (g *Git) noWorkflowChanges(ctx context.Context, review, span string) error {
	out, err := g.git(ctx, review, "log", "-m", "--name-only", "-z", "--no-renames",
		"--no-ext-diff", "--format=", span)
	if err != nil {
		return fmt.Errorf("%w: could not list changed files: %w", ErrHostCheck, err)
	}
	for _, path := range bytes.Split(out, []byte{0}) {
		path = bytes.Trim(path, "\n")
		// macOS is case-insensitive: .GitHub/Workflows is the same directory.
		p := strings.ToLower(string(path))
		if p == workflowsPath || strings.HasPrefix(p, workflowsPath+"/") {
			return fmt.Errorf("%w: the agent changed %s, which it may not touch",
				ErrHostCheck, string(path))
		}
	}
	return nil
}

// noSecrets looks for each secret in every commit's changes (merges included),
// message and author, so a value committed and removed again is still found. The
// value is never printed.
func (g *Git) noSecrets(
	ctx context.Context,
	review, span string,
	secrets []string,
) error {
	meta, err := g.git(ctx, review, "log", "--format=%H %an %ae %cn %ce%n%B", span)
	if err != nil {
		return fmt.Errorf("%w: could not read the commits: %w", ErrHostCheck, err)
	}
	for _, secret := range secrets {
		if len(secret) < minSecretLen {
			continue
		}
		if bytes.Contains(meta, []byte(secret)) {
			return fmt.Errorf("%w: a credential value appears in a commit message or author",
				ErrHostCheck)
		}
		out, logErr := g.git(ctx, review, "log", "-m", "--no-ext-diff", "--no-textconv",
			"--format=%H", "-S"+secret, span)
		if logErr != nil {
			return fmt.Errorf("%w: could not search the commits: %w", ErrHostCheck, logErr)
		}
		if hash := firstLine(strings.TrimSpace(string(out))); hash != "" {
			return fmt.Errorf("%w: a credential value appears in commit %s",
				ErrHostCheck, hash[:min(len(hash), shortHashLen)])
		}
	}
	return nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
