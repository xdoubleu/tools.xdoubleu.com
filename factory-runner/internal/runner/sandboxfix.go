package runner

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// sandboxFlakeFiles are the only files a sandbox-fix branch may change: the
// target repo's dev shell, whose lock is the pin.
const sandboxFlakeFiles = "flake.nix flake.lock"

// sandboxBranchPattern is the branch name an outcome's `Sandbox-fix:` line may
// name.
var sandboxBranchPattern = regexp.MustCompile(`^agent/sandbox-[a-z0-9][a-z0-9-]{0,60}$`)

// flakeProblem is why a sandbox-fix diff may not be published, or nil when it
// changes only flake.nix and flake.lock.
func flakeProblem(files []string) error {
	if len(files) == 0 {
		return errors.New("it changes nothing")
	}
	for _, f := range files {
		if !slices.Contains(strings.Fields(sandboxFlakeFiles), f) {
			return fmt.Errorf("it must change only %s, not %s",
				strings.ReplaceAll(sandboxFlakeFiles, " ", " and "), strings.Join(files, ", "))
		}
	}
	return nil
}

// CheckSandboxFix refuses a reviewed sandbox-fix branch whose net change from
// base is anything but the repo's flake.nix and flake.lock.
func (r *Reviewed) CheckSandboxFix(ctx context.Context, base string) error {
	net := "refs/heads/" + base + "...refs/heads/" + r.branch
	out, err := r.g.git(ctx, r.dir, "diff", "--name-only", "--no-renames",
		"--no-ext-diff", net)
	if err != nil {
		return fmt.Errorf("%w: could not list changed files: %w", ErrHostCheck, err)
	}
	var files []string
	for f := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	if perr := flakeProblem(files); perr != nil {
		return fmt.Errorf("%w: %w", ErrHostCheck, perr)
	}
	return nil
}

// publishSandboxFix pushes the flake-only branch the outcome names and
// opens its own pull request. It returns a line for the outcome and, when the
// branch itself was refused (name or files), why, so the agent can
// fix it. It never fails the job: the job's own work is published either way.
func (r *JobRunner) publishSandboxFix(
	ctx context.Context,
	job *ClaimedJob,
	cloneDir, text string,
	secrets []string,
) (string, string) {
	name, ok := findLine(text, "Sandbox-fix:")
	if !ok {
		return "", ""
	}
	if !sandboxBranchPattern.MatchString(name) {
		why := "the branch must be named agent/sandbox-<slug>."
		return "Sandbox fix not published: " + why, why
	}
	reviewed, err := r.Git.Inspect(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		name, job.ID+"-sandbox", cloneDir, "", secrets)
	if err == nil {
		err = reviewed.CheckSandboxFix(ctx, job.DefaultBranch)
	}
	if err != nil {
		if reviewed != nil {
			reviewed.Close()
		}
		why := strings.TrimPrefix(err.Error(), ErrHostCheck.Error()+": ")
		return "Sandbox fix not published: " + why, why
	}
	defer reviewed.Close()

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	token, err := r.pushToken(ctx, job.ID)
	if err != nil || token == "" {
		return "Sandbox fix not published: could not get a push token.", ""
	}
	remote := r.Git.RemoteURL(job.RepoOwner, job.RepoName)
	if err = reviewed.Push(ctx, remote, token); err != nil {
		r.Log.Warn("sandbox fix push failed", "job", job.ID,
			"error", scrub(err.Error(), token))
		return "Sandbox fix not published: the branch could not be pushed.", ""
	}
	problem, _ := findLine(text, "Blocked:")
	body := neutralize("The sandbox lacked a tool while running task `" + job.TaskID +
		"` (job `" + job.ID + "`): " +
		cmp.Or(problem, "see the job outcome") + "\n\nThis changes only the " +
		"repo's `flake.nix` and `flake.lock`. Its " +
		"`factory-sandbox` board task is named in the job's outcome.")
	url, err := r.openPullRequest(ctx, token, PullRequest{
		Owner: job.RepoOwner, Repo: job.RepoName, Head: name, Base: job.DefaultBranch,
		Title: "Sandbox: " + strings.ReplaceAll(
			strings.TrimPrefix(name, "agent/sandbox-"), "-", " "),
		Body: body,
	})
	if err != nil {
		r.Log.Warn("sandbox fix pull request failed", "job", job.ID,
			"error", scrub(err.Error(), token))
		return "Sandbox fix not published: the branch was pushed, but its pull request " +
			"could not be opened.", ""
	}
	return "Sandbox fix opened: " + url, ""
}

// sandboxRefusedFeedback tells the agent why its sandbox-fix branch was refused.
func sandboxRefusedFeedback(why string) string {
	return "The runner refused your sandbox fix: " + why + "\n\n" + sandboxFixRules +
		"\n\nFix the branch, commit, and write the outcome again."
}

// sandboxFixRules is what a sandbox fix may change.
const sandboxFixRules = "A sandbox fix changes only the repository's flake.nix and " +
	"flake.lock (create flake.nix with a devShell when the repository has none); " +
	"the lock pins the tools."
