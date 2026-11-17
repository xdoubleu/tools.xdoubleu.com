package diffcov

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// lineSet is the changed lines of a file; all marks an untracked file, which is
// new in its entirety and absent from git diff.
type lineSet struct {
	all   bool
	lines map[int]bool
}

func (s lineSet) has(n int) bool { return s.all || s.lines[n] }

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotepath=off"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

// resolveBase returns the ref to diff against: base when set, else the
// merge-base of origin/main (else the local main, for offline sandboxes) and
// HEAD, else origin/main.
func resolveBase(dir, base string) string {
	if base != "" {
		return base
	}
	for _, ref := range []string{"origin/main", "main"} {
		if out, err := git(dir, "merge-base", ref, "HEAD"); err == nil && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out)
		}
	}
	return "origin/main"
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// changedLines returns the lines added or modified under dir since base, keyed
// by dir-relative path and filtered by relevant, plus untracked files in full.
func changedLines(dir, base string, relevant func(string) bool) (map[string]lineSet, error) {
	if _, err := git(dir, "rev-parse", "--show-toplevel"); err != nil {
		return nil, fmt.Errorf("not a git repository")
	}
	diff, _ := git(dir, "diff", "-U0", "--relative", "--no-color", "--no-ext-diff", resolveBase(dir, base), "--", ".")
	changed := parseDiff(diff, relevant)

	untracked, _ := git(dir, "ls-files", "--others", "--exclude-standard")
	for _, f := range strings.Split(untracked, "\n") {
		if f != "" && relevant(f) {
			changed[f] = lineSet{all: true}
		}
	}
	return changed, nil
}

func parseDiff(diff string, relevant func(string) bool) map[string]lineSet {
	changed := map[string]lineSet{}
	current := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			current = ""
			if f, ok := strings.CutPrefix(line, "+++ b/"); ok && relevant(f) {
				current = f
			}
			continue
		}
		m := hunkRe.FindStringSubmatch(line)
		if m == nil || current == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		// A pure deletion hunk (+n,0) adds no lines to score.
		set, ok := changed[current]
		if !ok {
			set = lineSet{lines: map[int]bool{}}
		}
		for i := start; i < start+count; i++ {
			set.lines[i] = true
		}
		if count > 0 || ok {
			changed[current] = set
		}
	}
	return changed
}
