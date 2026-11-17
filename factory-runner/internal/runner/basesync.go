package runner

import (
	"bytes"
	"strings"
)

// exitConflict is the exit code of syncScript when the base conflicts.
const exitConflict = 3

// syncScript merges origin/$1 into the branch when the branch lacks its tip. It
// runs in the job VM, so the host never runs a merge in the agent's clone. A
// dirty tree is left alone; a conflict is aborted and reported as exitConflict.
const syncScript = `git merge-base --is-ancestor "origin/$1" HEAD 2>/dev/null && exit 0
git rev-parse --verify -q "origin/$1" >/dev/null || exit 0
git diff --quiet && git diff --cached --quiet || exit 0
git -c user.name=factory-runner -c user.email=factory-runner@localhost \
  merge --no-edit "origin/$1" >/dev/null 2>&1 && exit 0
git merge --abort
exit 3`

// generatedAdvice tells the agent how to resolve conflicts in generated stubs:
// buf.build is unreachable from the sandbox, so only the /local generators work.
const generatedAdvice = "For conflicts in generated stubs (api/gen, web/lib/gen) " +
	"take the base's version, then run `make proto/generate/local` in api/ and " +
	"`npm run generate:local` in web/ instead of hand-resolving; the plain " +
	"generators need buf.build, which the sandbox cannot reach."

// syncBase brings the base branch's tip into a branch that is behind it, so the
// gates run on a go.mod and toolchain pin that match the image. It returns the
// agent's instructions when the merge conflicts, else "".
func (l *jobLoop) syncBase() string {
	base := l.job.DefaultBranch
	err := l.r.Git.FetchBase(l.ctx, l.job.RepoOwner, l.job.RepoName, base,
		l.dirs.clone)
	if err != nil {
		l.r.Log.Warn("could not fetch the base branch", "job", l.job.ID,
			"error", scrub(err.Error(), l.token))
		return ""
	}
	var out bytes.Buffer
	code, err := l.session.Exec(l.ctx,
		[]string{bashCmd, "-c", syncScript, "sync", base}, "", &out)
	if err != nil || code != exitConflict {
		return ""
	}
	return strings.Join([]string{
		"The branch is behind " + base + " and merging origin/" + base +
			" conflicts. The host fetched origin/" + base + " into the clone.",
		"Merge origin/" + base + " into the current branch (a merge, not a " +
			"rebase: the runner pushes a fast-forward), resolve the conflicts " +
			"run the gates, commit, and write the outcome again.",
		generatedAdvice,
	}, " ")
}
