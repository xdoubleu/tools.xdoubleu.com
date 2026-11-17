package diffcov

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveBaseFallsBackToLocalMain(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "base")
	want := run("rev-parse", "HEAD")
	run("switch", "-q", "-c", "feature")
	run("commit", "-q", "--allow-empty", "-m", "work")

	if got := resolveBase(dir, ""); got != want {
		t.Errorf("resolveBase = %q, want %q", got, want)
	}
	if got := resolveBase(dir, "abc"); got != "abc" {
		t.Errorf("explicit base = %q", got)
	}
}
