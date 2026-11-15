package diffcov

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

type blockKey struct{ rng, stmts string }

// mergeProfiles takes the max count per block across profiles, skipping files
// that match skip. It returns the blocks, the mode line and the files read.
func mergeProfiles(paths []string, skip *matcher, warn io.Writer) (map[blockKey]int, string, int) {
	blocks := map[blockKey]int{}
	mode, valid := "", 0
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(warn, "Warning: coverage file not found: %s\n", p)
			continue
		}
		valid++
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if m, ok := strings.CutPrefix(line, "mode:"); ok {
				if mode == "" {
					mode = strings.TrimSpace(m)
				}
				continue
			}
			parts := strings.Fields(line)
			if len(parts) != 3 {
				continue
			}
			count, err := strconv.Atoi(parts[2])
			if err != nil {
				continue
			}
			file, _, _ := strings.Cut(parts[0], ":")
			if skip.match(file) {
				continue
			}
			k := blockKey{parts[0], parts[1]}
			if old, ok := blocks[k]; !ok || count > old {
				blocks[k] = count
			}
		}
		f.Close()
	}
	return blocks, mode, valid
}

type pkgStat struct{ covered, total int }

// packageStats counts each merged block's statements once, grouped by the
// directory of its file with prefix stripped.
func packageStats(blocks map[blockKey]int, prefix string) map[string]*pkgStat {
	stats := map[string]*pkgStat{}
	for k, count := range blocks {
		file, _, _ := strings.Cut(k.rng, ":")
		pkg := path.Dir(strings.TrimPrefix(file, prefix))
		n, err := strconv.Atoi(k.stmts)
		if err != nil {
			continue
		}
		s := stats[pkg]
		if s == nil {
			s = &pkgStat{}
			stats[pkg] = s
		}
		s.total += n
		if count > 0 {
			s.covered += n
		}
	}
	return stats
}

func printStats(w io.Writer, stats map[string]*pkgStat) {
	width := 10
	for pkg := range stats {
		width = max(width, len(pkg))
	}
	var covered, total int
	fmt.Fprintln(w)
	for _, pkg := range sortedKeys(stats) {
		s := stats[pkg]
		covered += s.covered
		total += s.total
		if s.total > 0 {
			fmt.Fprintf(w, "%-*s %5d / %-5d = %5.1f%%\n", width, pkg, s.covered, s.total, 100*float64(s.covered)/float64(s.total))
		}
	}
	fmt.Fprintln(w, strings.Repeat("-", width+25))
	if total == 0 {
		fmt.Fprintln(w, "combined: no statements")
		return
	}
	fmt.Fprintf(w, "%-*s %5d / %-5d = %5.1f%%\n", width, "combined", covered, total, 100*float64(covered)/float64(total))
}

func writeProfile(w io.Writer, blocks map[blockKey]int, mode string) {
	if mode == "" {
		mode = "set"
	}
	fmt.Fprintf(w, "mode: %s\n", mode)
	keys := make([]blockKey, 0, len(blocks))
	for k := range blocks {
		keys = append(keys, k)
	}
	sortBlocks(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s %s %d\n", k.rng, k.stmts, blocks[k])
	}
}
