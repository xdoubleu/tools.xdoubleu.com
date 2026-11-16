package services

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

var errNotPullRequest = errors.New("not a pull request address")

// selfCheckedRule names the rule a changed path falls under, or "" when it needs a
// person's check.
func selfCheckedRule(file string) string {
	file = strings.TrimPrefix(file, "/")
	// Paths whose change always needs a person's check, whatever else matches.
	for _, p := range []string{"infra/", "config/deploy", "web/"} {
		if strings.HasPrefix(file, p) {
			return ""
		}
	}
	if strings.HasPrefix(file, "migrations/") || strings.Contains(file, "/migrations/") {
		return ""
	}
	switch {
	case strings.HasSuffix(file, ".md") || strings.HasPrefix(file, "docs/"):
		return "docs"
	case hasAnySuffix(file, "_test.go", ".test.ts", ".test.tsx"):
		return "tests"
	case path.Base(file) == "Makefile" || strings.HasPrefix(file, ".github/") ||
		strings.HasPrefix(file, "tools/") || path.Base(file) == "Containerfile":
		return "CI/build"
	case strings.HasPrefix(file, "factory-runner/") ||
		strings.HasPrefix(file, "api/internal/platform/factory/"):
		return "factory"
	}
	return ""
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

// pathRule is the rule that sends the task of a merged pull request to Done by its
// changed files, or "" when it goes to Testing: also on a failed read.
func (s *JobService) pathRule(ctx context.Context, prURL string) string {
	if s.github == nil || prURL == "" {
		return ""
	}
	files, err := s.changedFiles(ctx, prURL)
	if err != nil {
		return ""
	}
	return selfCheckedBy(files)
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
		if t.Status != models.StatusTesting || t.PRState != githubapp.StateMerged {
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
