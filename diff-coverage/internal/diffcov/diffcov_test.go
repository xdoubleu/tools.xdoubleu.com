package diffcov

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlob(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"**/*.go", "a.go", true},
		{"**/*.go", "x/y/a.go", true},
		{"**/*.go", "a.ts", false},
		{"*.go", "x/a.go", true},
		{"gen/**", "gen/a/b.go", true},
		{"gen/**", "x/gen/b.go", false},
		{"**/gen/**", "x/gen/b.go", true},
		{"cmd/*/entry.go", "cmd/app/entry.go", true},
		{"cmd/*/entry.go", "cmd/a/b/entry.go", false},
		{"a?.go", "ab.go", true},
		{"a?.go", "a/.go", false},
		{"**/*_mock.go", "m/x_mock.go", true},
		{"a.b", "axb", false},
	}
	for _, c := range cases {
		m, err := newMatcher([]string{c.glob})
		if err != nil {
			t.Fatal(err)
		}
		if got := m.match(c.path); got != c.want {
			t.Errorf("%q vs %q = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
}

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,0 +2,2 @@
@@ -10 +12 @@
@@ -20,3 +21,0 @@
diff --git a/skip.txt b/skip.txt
--- a/skip.txt
+++ b/skip.txt
@@ -1 +1 @@
diff --git a/gone.go b/gone.go
--- a/gone.go
+++ /dev/null
@@ -1,3 +0,0 @@
diff --git a/del.go b/del.go
--- a/del.go
+++ b/del.go
@@ -5,2 +4,0 @@
`
	got := parseDiff(diff, func(p string) bool { return strings.HasSuffix(p, ".go") })
	if len(got) != 1 {
		t.Fatalf("files = %v", got)
	}
	for _, n := range []int{2, 3, 12} {
		if !got["a.go"].has(n) {
			t.Errorf("line %d missing", n)
		}
	}
	if got["a.go"].has(21) || got["a.go"].has(1) {
		t.Errorf("unexpected lines: %v", got["a.go"].lines)
	}
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func lines(ns ...int) lineSet {
	s := lineSet{lines: map[int]bool{}}
	for _, n := range ns {
		s.lines[n] = true
	}
	return s
}

func TestParseGoProfile(t *testing.T) {
	p := parseGoProfile(openFixture(t, "cover.out"), "example.com/mod/")
	if _, ok := p["../other/c.go"]; ok || len(p) != 2 {
		t.Fatalf("files = %v", p)
	}
	a := p["pkg/a.go"]
	// Block 5.20,7.2 ends at column 2 so line 7 is covered; 10.16 ends a
	// hit block and starts a missed one, so line 10 is partial.
	if c := a[7]; !c.hit || c.miss || !c.boundary {
		t.Errorf("line 7 = %+v", c)
	}
	if c := a[6]; !c.hit || c.boundary {
		t.Errorf("line 6 = %+v", c)
	}
	if c := a[10]; !c.hit || !c.miss {
		t.Errorf("line 10 = %+v", c)
	}
	if _, ok := p["pkg/b.go"][6]; !ok {
		t.Error("end column 2 reaches its end line")
	}
	if _, ok := parseGoProfile(strings.NewReader("mode: set\nx.go:1.1,3.1 1 1\n"), "")["x.go"][3]; ok {
		t.Error("end column 1 must not reach its end line")
	}
}

func TestReportGo(t *testing.T) {
	profile := parseGoProfile(openFixture(t, "cover.out"), "example.com/mod/")
	cases := []struct {
		name    string
		changed map[string]lineSet
		ok      bool
		want    []string
	}{
		{"covered", map[string]lineSet{"pkg/a.go": lines(5, 6, 7)}, true, []string{"100.0% of 3", "conservative:  66.7%"}},
		{"partial line gets no credit", map[string]lineSet{"pkg/a.go": lines(10)}, false, []string{"0.0% of 1", "1 file(s) below 80%"}},
		{"untracked counts whole file", map[string]lineSet{"pkg/b.go": {all: true}}, false, []string{"0.0% of 4"}},
		{"no data is not a failure", map[string]lineSet{"pkg/none.go": lines(1)}, true, []string{"no instrumented lines changed"}},
		{"comment-only change", map[string]lineSet{"pkg/a.go": lines(1, 2)}, true, []string{"no instrumented lines changed"}},
		{"at threshold passes", map[string]lineSet{"pkg/a.go": lines(5, 6, 7, 8, 9)}, true, []string{"100.0%"}},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if got := reportGo(&buf, c.changed, profile, 80, "origin/main"); got != c.ok {
			t.Errorf("%s: ok = %v\n%s", c.name, got, buf.String())
		}
		for _, w := range c.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: output lacks %q\n%s", c.name, w, buf.String())
			}
		}
	}
}

func TestReportLcov(t *testing.T) {
	files := parseLcov(openFixture(t, "web.lcov"))
	// lib/analytics.ts: line 5 hit, line 6 missed, branch 5 half taken.
	cases := []struct {
		name    string
		changed map[string]lineSet
		thresh  float64
		ok      bool
		want    string
	}{
		{"line and branch below", map[string]lineSet{"lib/analytics.ts": lines(5, 6)}, 80, false, "50.0% lines,  50.0% branches"},
		{"hit line only", map[string]lineSet{"lib/analytics.ts": lines(5)}, 80, false, "100.0% lines,  50.0% branches"},
		{"branch ok at lower threshold", map[string]lineSet{"lib/analytics.ts": lines(5)}, 50, true, "meet the 50%"},
		{"missed line only", map[string]lineSet{"lib/analytics.ts": lines(6)}, 80, false, "0.0% lines,   --  branches"},
		{"no data fails", map[string]lineSet{"lib/new.ts": {all: true}}, 80, false, "no coverage data found"},
		{"nothing instrumented", map[string]lineSet{"lib/analytics.ts": lines(1)}, 80, true, "no instrumented lines changed"},
		{"untracked scores whole file", map[string]lineSet{"lib/analytics.ts": {all: true}}, 80, false, "(2 changed lines)"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if got := reportLcov(&buf, c.changed, files, c.thresh, "origin/main"); got != c.ok {
			t.Errorf("%s: ok = %v\n%s", c.name, got, buf.String())
		}
		if !strings.Contains(buf.String(), c.want) {
			t.Errorf("%s: output lacks %q\n%s", c.name, c.want, buf.String())
		}
	}
}

func TestParseLcovDuplicatesAndBadRecords(t *testing.T) {
	in := "SF:a.ts\nDA:1,0\nDA:1,3\nDA:x,1\nDA:2\nBRDA:1,0,0,-\nBRDA:bad\nBRDA:q,0,0,1\nend_of_record\nDA:9,9\n"
	f := parseLcov(strings.NewReader(in))["a.ts"]
	if f.lines[1] != 3 || len(f.lines) != 1 {
		t.Errorf("lines = %v", f.lines)
	}
	if len(f.branches[1]) != 1 || f.branches[1][0] != 0 {
		t.Errorf("branches = %v", f.branches)
	}
}

func TestNormalizeLcovPaths(t *testing.T) {
	got := normalizeLcovPaths(map[string]*lcovFile{"/p/web/a.ts": {}, "b.ts": {}}, "/p/web")
	if got["a.ts"] == nil || got["b.ts"] == nil {
		t.Errorf("paths = %v", got)
	}
}

func TestMerge(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a.out", "mode: set\nm/p/x.go:1.1,2.2 2 1\nm/p/x.go:3.1,4.2 1 0\nm/gen/g.go:1.1,2.2 5 1\nm/p/x_mock.go:1.1,2.2 5 1\n")
	b := write("b.out", "mode: set\nm/p/x.go:3.1,4.2 1 4\nm/q/y.go:1.1,2.2 1 0\nbad line\nm/q/y.go:9.1,9.2 1 z\n")
	out := filepath.Join(dir, "merged.out")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"merge", "-o", out, a, b, filepath.Join(dir, "missing.out")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code %d: %s", code, stderr.String())
	}
	for _, w := range []string{"m/p", "3 / 3", "m/q", "0 / 1", "combined", "3 / 4", " 75.0%"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\n%s", w, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "missing.out") {
		t.Errorf("stderr = %q", stderr.String())
	}
	merged, _ := os.ReadFile(out)
	if string(merged) != "mode: set\nm/p/x.go:1.1,2.2 2 1\nm/p/x.go:3.1,4.2 1 4\nm/q/y.go:1.1,2.2 1 0\n" {
		t.Errorf("merged = %q", merged)
	}
	if Run([]string{"merge", filepath.Join(dir, "none")}, &stdout, &stderr) != 1 {
		t.Error("no valid files must fail")
	}
	if Run([]string{"merge"}, &stdout, &stderr) != 1 {
		t.Error("no args must fail")
	}
	empty := write("e.out", "mode: set\n")
	stdout.Reset()
	if Run([]string{"merge", empty}, &stdout, &stderr) != 0 || !strings.Contains(stdout.String(), "no statements") {
		t.Errorf("empty merge: %s", stdout.String())
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gate(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestGoGateEndToEnd gates a real git diff in a subdirectory of a repo.
func TestGoGateEndToEnd(t *testing.T) {
	repo := t.TempDir()
	run(t, repo, "git", "init", "-q", "-b", "main")
	mod := filepath.Join(repo, "svc")
	writeFile(t, filepath.Join(mod, "go.mod"), "module example.com/mod\n\ngo 1.24\n")
	writeFile(t, filepath.Join(mod, "pkg", "a.go"), "package pkg\n\nfunc A() {\n\tprintln(1)\n\tprintln(2)\n}\n")
	writeFile(t, filepath.Join(repo, "outside.go"), "package x\n")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-qm", "base")
	base := run(t, repo, "git", "rev-parse", "HEAD")

	profile := filepath.Join(mod, "cover.out")
	writeFile(t, profile, "mode: set\nexample.com/mod/pkg/a.go:3.10,6.2 3 1\nexample.com/mod/pkg/n.go:3.10,5.2 2 0\n")

	// No relevant change yet.
	if code, out := gate(t, mod, "go", "--base", base, profile); code != 0 || !strings.Contains(out, "No coverage-relevant") {
		t.Fatalf("clean: %d %s", code, out)
	}

	// A covered edit, plus an edit outside the module and an excluded mock.
	writeFile(t, filepath.Join(mod, "pkg", "a.go"), "package pkg\n\nfunc A() {\n\tprintln(1)\n\tprintln(3)\n}\n")
	writeFile(t, filepath.Join(repo, "outside.go"), "package y\n")
	writeFile(t, filepath.Join(mod, "pkg", "x_mock.go"), "package pkg\n")
	if code, out := gate(t, mod, "go", "--base", base, profile); code != 0 || !strings.Contains(out, "pkg/a.go") || strings.Contains(out, "outside") || strings.Contains(out, "x_mock") {
		t.Fatalf("covered edit: %d %s", code, out)
	}

	// An untracked new file with no coverage fails and counts whole.
	writeFile(t, filepath.Join(mod, "pkg", "n.go"), "package pkg\n\nfunc N() {\n\tprintln(1)\n}\n")
	if code, out := gate(t, mod, "go", "--base", base, profile); code != 1 || !strings.Contains(out, "pkg/n.go") || !strings.Contains(out, "0.0% of 3") {
		t.Fatalf("untracked: %d %s", code, out)
	}
	// Excluding it, or lowering the threshold to 0, passes.
	if code, out := gate(t, mod, "go", "--base", base, "--exclude", "**/n.go", profile); code != 0 {
		t.Fatalf("exclude: %d %s", code, out)
	}
	if code, out := gate(t, mod, "go", "--base", base, "--threshold", "0", profile); code != 0 {
		t.Fatalf("threshold: %d %s", code, out)
	}
	if code, out := gate(t, mod, "go", "--base", base, "--include", "pkg/a.go", profile); code != 0 {
		t.Fatalf("include: %d %s", code, out)
	}
}

func TestLcovGateEndToEnd(t *testing.T) {
	repo := t.TempDir()
	run(t, repo, "git", "init", "-q", "-b", "main")
	web := filepath.Join(repo, "web")
	writeFile(t, filepath.Join(web, "lib", "analytics.ts"), "// head\nexport function track() {\n  if (x) {\n    a()\n  }\n  b()\n}\n")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-qm", "base")
	base := run(t, repo, "git", "rev-parse", "HEAD")

	lcov := filepath.Join(web, "lcov.info")
	writeFile(t, lcov, "SF:lib/analytics.ts\nDA:5,1\nDA:6,0\nBRDA:5,0,0,1\nBRDA:5,0,1,0\nend_of_record\n")
	writeFile(t, filepath.Join(web, "lib", "analytics.ts"), "// head\nexport function track() {\n  if (x) {\n    a()\n  }\n  c()\n}\n")
	if code, out := gate(t, web, "lcov", "--base", base, lcov); code != 1 || !strings.Contains(out, "0.0% lines") {
		t.Fatalf("missed line: %d %s", code, out)
	}
	writeFile(t, filepath.Join(web, "lib", "new.ts"), "export const a = 1\n")
	writeFile(t, filepath.Join(web, "lib", "new.test.ts"), "x\n")
	writeFile(t, filepath.Join(web, "lib", "t.d.ts"), "x\n")
	code, out := gate(t, web, "lcov", "--base", base, "--threshold", "0", lcov)
	if code != 1 || !strings.Contains(out, "lib/new.ts") || !strings.Contains(out, "no coverage data found") || strings.Contains(out, "new.test") || strings.Contains(out, "t.d.ts") {
		t.Fatalf("new file: %d %s", code, out)
	}
}

func TestRunErrors(t *testing.T) {
	var out, errb bytes.Buffer
	for _, args := range [][]string{{}, {"nope"}, {"go"}, {"go", "--bogus", "x"}, {"lcov", "/does/not/exist"}, {"go", "--include", "[", "x"}} {
		out.Reset()
		errb.Reset()
		code := Run(args, &out, &errb)
		if code == 0 {
			t.Errorf("%v: code 0", args)
		}
	}
	if Run([]string{"help"}, &out, &errb) != 0 {
		t.Error("help must succeed")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if code, o := gate(t, dir, "go", filepath.Join("testdata", "x")); code == 0 {
		t.Errorf("missing report: %s", o)
	}
	writeFile(t, "r.out", "mode: set\n")
	if code, o := gate(t, dir, "go", "r.out"); code != 1 || !strings.Contains(o, "git") {
		t.Errorf("not a repo: %d %s", code, o)
	}
}

func TestModuleName(t *testing.T) {
	dir := t.TempDir()
	if _, err := moduleName(dir); err == nil {
		t.Error("missing go.mod must fail")
	}
	writeFile(t, filepath.Join(dir, "go.mod"), "go 1.24\n")
	if _, err := moduleName(dir); err == nil {
		t.Error("no module line must fail")
	}
	writeFile(t, filepath.Join(dir, "go.mod"), "// c\nmodule \"a.b/c\"\n")
	if m, _ := moduleName(dir); m != "a.b/c" {
		t.Errorf("module = %q", m)
	}
}
