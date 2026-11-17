// Package diffcov gates the coverage of changed lines from Go coverage
// profiles and lcov reports.
package diffcov

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const usage = `Usage:
  diff-coverage go   [flags] <coverage.out>   gate a go test -coverprofile
  diff-coverage lcov [flags] <lcov.info>      gate an lcov report (lines and branches)
  diff-coverage merge [-o merged.out] [--exclude glob] <coverage.out>...

Run in the module (go) or project (lcov) directory the report paths are
relative to. Flags for go and lcov:
  --threshold N   minimum percentage on changed lines (default 80)
  --base REF      diff against REF (default: merge-base of origin/main, else main, and HEAD)
  --include GLOB  only gate matching files (repeatable)
  --exclude GLOB  skip matching files (repeatable; replaces the defaults)
`

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

var (
	goInclude   = []string{"**/*.go"}
	goExclude   = []string{"**/*_mock.go", "**/gen/**"}
	lcovInclude = []string{"**/*.ts", "**/*.tsx"}
	lcovExclude = []string{"**/*.d.ts", "**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"}
)

// Run executes the CLI and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	ok := true
	switch args[0] {
	case "go", "lcov":
		ok, err = runGate(args[0], args[1:], stdout)
	case "merge":
		err = runMerge(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !ok {
		return 1
	}
	return 0
}

func runGate(kind string, args []string, stdout io.Writer) (bool, error) {
	fs := flag.NewFlagSet(kind, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	threshold := fs.Float64("threshold", 80, "")
	base := fs.String("base", "", "")
	var include, exclude stringList
	fs.Var(&include, "include", "")
	fs.Var(&exclude, "exclude", "")
	if err := fs.Parse(args); err != nil {
		return false, fmt.Errorf("%w\n\n%s", err, usage)
	}
	if fs.NArg() != 1 {
		return false, fmt.Errorf("expected one report file\n\n%s", usage)
	}
	report := fs.Arg(0)
	dir, err := os.Getwd()
	if err != nil {
		return false, err
	}

	defInclude, defExclude := goInclude, goExclude
	if kind == "lcov" {
		defInclude, defExclude = lcovInclude, lcovExclude
	}
	if len(include) == 0 {
		include = defInclude
	}
	if len(exclude) == 0 {
		exclude = defExclude
	}
	inc, err := newMatcher(include)
	if err != nil {
		return false, err
	}
	exc, err := newMatcher(exclude)
	if err != nil {
		return false, err
	}
	relevant := func(p string) bool {
		if kind == "go" && strings.HasSuffix(p, "_test.go") {
			return false
		}
		return inc.match(p) && !exc.match(p)
	}

	f, err := os.Open(report)
	if err != nil {
		return false, fmt.Errorf("%s not found: generate the coverage report first", report)
	}
	defer f.Close()

	changed, err := changedLines(dir, *base, relevant)
	if err != nil {
		return false, err
	}
	if len(changed) == 0 {
		fmt.Fprintln(stdout, "No coverage-relevant files changed.")
		return true, nil
	}
	label := "origin/main"
	if *base != "" {
		label = *base
	}
	if kind == "go" {
		mod, err := moduleName(dir)
		if err != nil {
			return false, err
		}
		return reportGo(stdout, changed, parseGoProfile(f, mod+"/"), *threshold, label), nil
	}
	return reportLcov(stdout, changed, normalizeLcovPaths(parseLcov(f), dir), *threshold, label), nil
}

// normalizeLcovPaths rewrites absolute SF paths under dir to relative ones.
func normalizeLcovPaths(files map[string]*lcovFile, dir string) map[string]*lcovFile {
	out := make(map[string]*lcovFile, len(files))
	for p, f := range files {
		out[strings.TrimPrefix(p, strings.TrimSuffix(dir, "/")+"/")] = f
	}
	return out
}

func runMerge(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("o", "", "")
	var exclude stringList
	fs.Var(&exclude, "exclude", "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n\n%s", err, usage)
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("expected at least one coverage file\n\n%s", usage)
	}
	if len(exclude) == 0 {
		exclude = goExclude
	}
	skip, err := newMatcher(exclude)
	if err != nil {
		return err
	}
	blocks, mode, valid := mergeProfiles(fs.Args(), skip, stderr)
	if valid == 0 {
		return fmt.Errorf("Error: no valid coverage files found")
	}
	prefix := ""
	if dir, err := os.Getwd(); err == nil {
		if mod, err := moduleName(dir); err == nil {
			prefix = mod + "/"
		}
	}
	printStats(stdout, packageStats(blocks, prefix))
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		writeProfile(f, blocks, mode)
	}
	return nil
}

func sortBlocks(keys []blockKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].rng != keys[j].rng {
			return keys[i].rng < keys[j].rng
		}
		return keys[i].stmts < keys[j].stmts
	})
}
