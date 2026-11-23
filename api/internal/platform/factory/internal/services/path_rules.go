package services

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

var errNotPullRequest = errors.New("not a pull request address")

// selfCheckedRule names the rule a changed path falls under, or "" when it needs a
// person's check.
func selfCheckedRule(file string) string {
	file = strings.TrimPrefix(file, "/")
	// Factory paths under otherwise denied prefixes: its generated code follows
	// the proto, and its migrations run first at startup and stop it on failure.
	if hasAnyPrefix(file, "proto/factory/", "api/gen/factory/", "web/lib/gen/factory/",
		"api/internal/platform/factory/migrations/") {
		return "factory"
	}
	// Paths whose change always needs a person's check, whatever else matches.
	for _, p := range []string{"infra/", "config/deploy", "web/"} {
		if strings.HasPrefix(file, p) {
			return ""
		}
	}
	if strings.HasPrefix(file, "migrations/") || strings.Contains(file, "/migrations/") {
		return ""
	}
	if isADR(file) {
		return ""
	}
	switch {
	case strings.HasSuffix(file, ".md") || strings.HasPrefix(file, "docs/"):
		return "docs"
	case hasAnySuffix(file, "_test.go", ".test.ts", ".test.tsx"):
		return "tests"
	case path.Base(file) == "Makefile" || strings.HasPrefix(file, ".github/") ||
		strings.HasPrefix(file, "tools/") || path.Base(file) == "Containerfile" ||
		path.Base(file) == "flake.nix" || path.Base(file) == "flake.lock":
		return "CI/build"
	case hasAnyPrefix(file, "factory-runner/", "api/internal/platform/factory/",
		"api/internal/platform/githubapp/"):
		return "factory"
	case hasAnyPrefix(file, "diff-coverage/"):
		return "CI/build"
	case isAgentTooling(file):
		return "agent tooling"
	}
	return ""
}

// isADR reports whether file is a decision record, which the owner reviews.
func isADR(file string) bool {
	dir, base := path.Split(strings.TrimPrefix(file, "/"))
	return dir == "docs/" && strings.HasPrefix(base, "adr-") &&
		strings.HasSuffix(base, ".md")
}

// adrFiles are the decision records among the changed files.
func adrFiles(files []string) []string {
	var adrs []string
	for _, f := range files {
		if isADR(f) {
			adrs = append(adrs, path.Base(f))
		}
	}
	return adrs
}

// isAgentTooling is agent instructions and the scripts only CI runs; other
// scripts/ files, such as the deploy scripts, need a person's check.
func isAgentTooling(file string) bool {
	if file == "AGENTS.md" || file == "CLAUDE.md" ||
		hasAnyPrefix(file, ".claude/", ".factory/", "api/scripts/") {
		return true
	}
	dir, base := path.Split(file)
	return dir == "scripts/" && strings.HasSuffix(base, ".sh") &&
		hasAnyPrefix(base, "lint_", "test_", "check_")
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

// selfCheckedBy reports the rules that cover every changed file, joined, or ""
// when there are no files or any file needs a person's check.
func selfCheckedBy(files []string) string {
	var rules []string
	seen := map[string]bool{}
	for _, f := range files {
		rule := selfCheckedRule(f)
		if rule == "" {
			return ""
		}
		if !seen[rule] {
			seen[rule] = true
			rules = append(rules, rule)
		}
	}
	return strings.Join(rules, ", ")
}

// changedFiles reads the files of the pull request at prURL through the GitHub
// App.
func (s *JobService) changedFiles(ctx context.Context, prURL string) ([]string, error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return nil, err
	}
	m := pullPath.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, errNotPullRequest
	}
	number, err := strconv.Atoi(m[3])
	if err != nil {
		return nil, err
	}
	return s.github.PullRequestFiles(ctx, m[1], m[2], number)
}

// mergedFiles are the changed files of a merged pull request, or nil when they
// can't be read: the task then goes to Testing.
func (s *JobService) mergedFiles(ctx context.Context, prURL string) []string {
	if s.github == nil || prURL == "" {
		return nil
	}
	files, err := s.changedFiles(ctx, prURL)
	if err != nil {
		return nil
	}
	return files
}

// BackfillPathRules moves, once, each Testing task whose merged pull request
// the path rules cover to Done, with a comment naming the rule. Once it ran,
// a task moved back to Testing stays there. A failed read leaves the task and
// repeats the whole pass on the next poll.
func (s *JobService) BackfillPathRules(ctx context.Context) (int, error) {
	if s.github == nil {
		return 0, nil
	}
	if done, err := s.settings.PathRulesBackfilled(ctx); err != nil || done {
		return 0, err
	}
	tasks, err := s.tasks.List(ctx, "")
	if err != nil {
		return 0, err
	}
	moved := 0
	var failures []error
	for i := range tasks {
		t := &tasks[i]
		if t.Status != models.StatusTesting || t.PRState != StateMerged {
			continue
		}
		files, readErr := s.changedFiles(ctx, t.PRURL)
		if readErr != nil {
			failures = append(failures, readErr)
			continue
		}
		rule := selfCheckedBy(files)
		if rule == "" {
			continue
		}
		if _, err = s.tasks.Move(ctx, t.ID, models.StatusDone); err != nil {
			failures = append(failures, err)
			continue
		}
		body := "Moved to Done: every file of the merged pull request is checked by CI or " +
			"the board (" + rule + " rule): " + t.PRURL
		if _, err = s.tasks.AddComment(ctx, t.ID, systemAuthor, body); err != nil {
			failures = append(failures, err)
		}
		moved++
	}
	if len(failures) > 0 {
		return moved, errors.Join(failures...)
	}
	return moved, s.settings.SetPathRulesBackfilled(ctx)
}
