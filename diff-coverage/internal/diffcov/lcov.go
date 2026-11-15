package diffcov

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type lcovFile struct {
	lines    map[int]int
	branches map[int][]int
}

// parseLcov reads DA: and BRDA: records keyed by SF path, keeping line
// numbers so coverage can be scoped to the diff. A BRDA taken count of "-"
// means the branch was never reached.
func parseLcov(r io.Reader) map[string]*lcovFile {
	files := map[string]*lcovFile{}
	var cur *lcovFile
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			cur = &lcovFile{lines: map[int]int{}, branches: map[int][]int{}}
			files[line[3:]] = cur
		case strings.HasPrefix(line, "DA:") && cur != nil:
			parts := strings.Split(line[3:], ",")
			if len(parts) < 2 {
				continue
			}
			n, err1 := strconv.Atoi(parts[0])
			hits, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				continue
			}
			// Some reporters emit a line twice; keep the best count.
			cur.lines[n] = max(cur.lines[n], hits)
		case strings.HasPrefix(line, "BRDA:") && cur != nil:
			parts := strings.Split(line[5:], ",")
			if len(parts) < 4 {
				continue
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			hits := 0
			if parts[3] != "-" {
				hits, _ = strconv.Atoi(parts[3])
			}
			cur.branches[n] = append(cur.branches[n], hits)
		case line == "end_of_record":
			cur = nil
		}
	}
	return files
}

func pct(covered, total int) (float64, bool) {
	if total == 0 {
		return 0, false
	}
	return float64(covered) / float64(total) * 100, true
}

func countHit(hits []int) int {
	n := 0
	for _, h := range hits {
		if h > 0 {
			n++
		}
	}
	return n
}

// reportLcov gates both line and branch coverage of the changed lines. A
// changed file with no coverage record at all fails.
func reportLcov(w io.Writer, changed map[string]lineSet, files map[string]*lcovFile, threshold float64, baseLabel string) bool {
	fmt.Fprintf(w, "Diff coverage check (changed lines vs %s, threshold %g%%)\n\n", baseLabel, threshold)
	flagged := 0
	paths := sortedKeys(changed)
	for _, path := range paths {
		data := files[path]
		if data == nil {
			fmt.Fprintf(w, "  ✗ %-50s no coverage data found\n", path)
			flagged++
			continue
		}
		set := changed[path]
		var lines, branches []int
		for n, h := range data.lines {
			if set.has(n) {
				lines = append(lines, h)
			}
		}
		for n, hs := range data.branches {
			if set.has(n) {
				branches = append(branches, hs...)
			}
		}
		// Comments, imports and type-only lines carry no DA:/BRDA: record.
		if len(lines) == 0 && len(branches) == 0 {
			fmt.Fprintf(w, "  -  %-50s no instrumented lines changed\n", path)
			continue
		}
		linePct, lineOK := pct(countHit(lines), len(lines))
		branchPct, branchOK := pct(countHit(branches), len(branches))
		lineStr, branchStr := "  --  lines", "  --  branches"
		if lineOK {
			lineStr = fmt.Sprintf("%5.1f%% lines", linePct)
		}
		if branchOK {
			branchStr = fmt.Sprintf("%5.1f%% branches", branchPct)
		}
		mark := "✓"
		if (lineOK && linePct < threshold) || (branchOK && branchPct < threshold) {
			mark = "✗"
			flagged++
		}
		fmt.Fprintf(w, "  %s %-50s %s, %s (%d changed lines)\n", mark, path, lineStr, branchStr, len(lines))
	}
	fmt.Fprintln(w)
	if flagged > 0 {
		fmt.Fprintf(w, "%d/%d file(s) below %g%% threshold on changed lines.\n", flagged, len(paths), threshold)
		return false
	}
	fmt.Fprintf(w, "All %d changed file(s) meet the %g%% threshold on changed lines.\n", len(paths), threshold)
	return true
}
