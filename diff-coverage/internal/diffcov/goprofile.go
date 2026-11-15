package diffcov

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// lineCov is the coverage of one source line over every block touching it.
// boundary reports a hit block starting or ending on the line.
type lineCov struct {
	hit, miss, boundary bool
}

var blockRe = regexp.MustCompile(`^(\S+):(\d+)\.\d+,(\d+)\.(\d+) \d+ (\d+)$`)

// moduleName reads the module path from go.mod in dir.
func moduleName(dir string) (string, error) {
	data, err := os.ReadFile(dir + "/go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", fmt.Errorf("go.mod has no module line")
}

// parseGoProfile returns per-line coverage keyed by module-relative path. A
// line hosting blocks that disagree is partial: both hit and miss are set.
func parseGoProfile(r io.Reader, prefix string) map[string]map[int]lineCov {
	files := map[string]map[int]lineCov{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	sc.Scan() // "mode: ..." header
	for sc.Scan() {
		m := blockRe.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil || !strings.HasPrefix(m[1], prefix) {
			continue
		}
		rel := m[1][len(prefix):]
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		endCol, _ := strconv.Atoi(m[4])
		count, _ := strconv.Atoi(m[5])
		// A block ending at column 1 doesn't actually reach its end line.
		last := end
		if endCol == 1 {
			last = end - 1
		}
		lines := files[rel]
		if lines == nil {
			lines = map[int]lineCov{}
			files[rel] = lines
		}
		for n := start; n <= last; n++ {
			c := lines[n]
			if count > 0 {
				c.hit = true
				if n == start || n == last {
					c.boundary = true
				}
			} else {
				c.miss = true
			}
			lines[n] = c
		}
	}
	return files
}

// reportGo prints the per-file verdict and reports whether every file met the
// threshold. The gate is the primary percentage; the conservative one counts
// only block start/end lines of hit blocks.
func reportGo(w io.Writer, changed map[string]lineSet, profile map[string]map[int]lineCov, threshold float64, baseLabel string) bool {
	fmt.Fprintf(w, "Diff coverage check (changed lines vs %s, threshold %g%%)\n\n", baseLabel, threshold)
	type failure struct {
		path  string
		pct   float64
		total int
	}
	var failing []failure
	for _, path := range sortedKeys(changed) {
		fileLines := profile[path]
		set := changed[path]
		instrumented := map[int]lineCov{}
		for n, c := range fileLines {
			if set.has(n) {
				instrumented[n] = c
			}
		}
		if len(instrumented) == 0 {
			fmt.Fprintf(w, "  -  %-60s no instrumented lines changed\n", path)
			continue
		}
		covered, conservative := 0, 0
		for _, c := range instrumented {
			if c.hit && !c.miss {
				covered++
				if c.boundary {
					conservative++
				}
			}
		}
		total := len(instrumented)
		pct := float64(covered) / float64(total) * 100
		consPct := float64(conservative) / float64(total) * 100
		mark := "✓"
		if pct < threshold {
			mark = "✗"
			failing = append(failing, failure{path, pct, total})
		}
		fmt.Fprintf(w, "  %s %-60s %5.1f%% of %d changed lines  (conservative: %5.1f%%)\n", mark, path, pct, total, consPct)
	}
	fmt.Fprintln(w)
	fmt.Fprint(w, "The gate uses the primary percentage; the conservative percentage\n"+
		"counts only block start/end lines of hit blocks, so a large gap\n"+
		"between them means the primary result is optimistic.\n")
	if len(failing) > 0 {
		for _, f := range failing {
			fmt.Fprintf(w, "  ✗ %s  %.1f%% of %d changed lines\n", f.path, f.pct, f.total)
		}
		fmt.Fprintf(w, "%d file(s) below %g%% threshold on changed lines.\n", len(failing), threshold)
		return false
	}
	fmt.Fprintf(w, "All changed files meet the %g%% threshold on changed lines.\n", threshold)
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
