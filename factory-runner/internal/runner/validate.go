package runner

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// BranchPrefix is the only namespace an agent may push to.
const BranchPrefix = "agent/"

var (
	repoPattern = regexp.MustCompile(
		`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`,
	)
	branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

	// ErrRefused marks a job the runner won't run.
	ErrRefused = errors.New("job refused")
)

// Job is what the server asks the runner to run. Every field is untrusted: a
// compromised server must not reach outside the runner's own allowlist.
type Job struct {
	Repo       string
	Branch     string
	BaseBranch string
}

// ValidRepo reports whether s is a plain owner/name.
func ValidRepo(s string) bool {
	return repoPattern.MatchString(s)
}

// validRef checks a git ref name conservatively: no leading dash (an option to
// git), no "..", no empty or dot-prefixed path parts, no ".lock" suffix.
func validRef(ref string) bool {
	if !branchPattern.MatchString(ref) ||
		strings.Contains(ref, "..") ||
		strings.HasSuffix(ref, "/") ||
		strings.HasSuffix(ref, ".lock") ||
		strings.HasSuffix(ref, ".") {
		return false
	}
	for part := range strings.SplitSeq(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// Validate refuses a job whose repo is outside allowed, whose branch is outside
// agent/*, or whose values could be read as options.
func Validate(job Job, allowed []string) error {
	if !ValidRepo(job.Repo) {
		return fmt.Errorf("%w: invalid repo %q", ErrRefused, job.Repo)
	}
	if !allowedRepo(job.Repo, allowed) {
		return fmt.Errorf("%w: repo %q is not in the allowlist", ErrRefused, job.Repo)
	}
	if !strings.HasPrefix(job.Branch, BranchPrefix) || !validRef(job.Branch) {
		return fmt.Errorf(
			"%w: branch %q is outside %s*",
			ErrRefused,
			job.Branch,
			BranchPrefix,
		)
	}
	if !validRef(job.BaseBranch) {
		return fmt.Errorf("%w: invalid base branch %q", ErrRefused, job.BaseBranch)
	}
	return nil
}

func allowedRepo(repo string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, repo) {
			return true
		}
	}
	return false
}
