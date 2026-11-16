package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// causePush marks a result whose push already failed.
const causePush = "push"

// Notes on where a job's work came from, for the board and the agent.
const (
	noteFromMac = "The earlier work of this task was never pushed; it was taken over " +
		"from an earlier job's clone on this Mac."
	noteFresh = "The earlier work of this task is neither on GitHub nor in a clone on " +
		"this Mac, so this job starts again from the base branch."
)

// holdsBranch says whether a job clone has branch checked out or stored.
func holdsBranch(clone, branch string) bool {
	loose := filepath.Join(clone, ".git", "refs", "heads", branch)
	if _, err := os.Stat(loose); err == nil {
		return true
	}
	packed, err := os.ReadFile(filepath.Join(clone, ".git", "packed-refs"))
	return err == nil && bytes.Contains(packed, []byte(" refs/heads/"+branch+"\n"))
}

// strandedClone is the newest job clone on this Mac that holds branch, or "".
// Job directories are never pruned, so a job that ended without a pushed branch
// leaves its work here.
func (r *JobRunner) strandedClone(branch string) string {
	entries, err := os.ReadDir(filepath.Join(r.DataDir, "jobs"))
	if err != nil {
		return ""
	}
	var best string
	var bestAt time.Time
	for _, e := range entries {
		if !jobIDPattern.MatchString(e.Name()) {
			continue
		}
		clone := filepath.Join(r.DataDir, "jobs", e.Name(), "clone")
		info, statErr := os.Stat(clone)
		if statErr != nil || !holdsBranch(clone, branch) {
			continue
		}
		if best == "" || info.ModTime().After(bestAt) {
			best, bestAt = clone, info.ModTime()
		}
	}
	return best
}

// setAsideClone moves the job's own clone out of its directory when it holds the
// branch, so clearing the directory (a job claimed again) never loses unpushed
// work. It returns where the clone went, or "".
func (r *JobRunner) setAsideClone(job *ClaimedJob, clone string) string {
	if job.Routine != "" || !holdsBranch(clone, job.Branch) {
		return ""
	}
	kept := filepath.Join(r.DataDir, "jobs", ".kept-"+job.ID)
	if removeAll(kept) != nil || os.Rename(clone, kept) != nil {
		return ""
	}
	return kept
}

// moveClone puts the stranded clone at dest.
func moveClone(from, dest string) bool {
	if os.MkdirAll(filepath.Dir(dest), gitDirPerm) != nil {
		return false
	}
	return os.Rename(from, dest) == nil
}

// pushStranded pushes the branch of a job that ended without a pull request, once
// the host's checks pass, so the work is on GitHub for any later job. It returns a
// line for the outcome, or "" when nothing was pushed.
func (r *JobRunner) pushStranded(
	ctx context.Context,
	job *ClaimedJob,
	cloneDir, since string,
	secrets []string,
) string {
	if r.GitHub == nil || job.Routine != "" || ctx.Err() != nil {
		return ""
	}
	reviewed, err := r.Git.Inspect(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		job.Branch, job.ID, cloneDir, since, secrets)
	if err != nil {
		return ""
	}
	defer reviewed.Close()

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	token, err := r.pushToken(ctx, job.ID)
	if err != nil || token == "" {
		return ""
	}
	remote := r.Git.RemoteURL(job.RepoOwner, job.RepoName)
	if err = reviewed.Push(ctx, remote, token); err != nil {
		r.Log.Warn("could not push the stranded branch", "job", job.ID,
			"error", scrub(err.Error(), token))
		return ""
	}
	return fmt.Sprintf("The work is on the branch %s on GitHub, without a pull request.",
		job.Branch)
}

// errNoClone says there was no earlier clone to take over.
var errNoClone = errors.New("no earlier clone")

// takeStranded makes cloneDir from the clone kept aside or the newest stranded
// one, when the branch is not on GitHub.
func (r *JobRunner) takeStranded(job *ClaimedJob, cloneDir, kept string) error {
	from := kept
	if from == "" {
		from = r.strandedClone(job.Branch)
	}
	if from == "" || !strings.HasPrefix(from, r.DataDir) || !moveClone(from, cloneDir) {
		return errNoClone
	}
	return nil
}
