package services

import (
	"sort"
)

const (
	// asideAdjacencyRatio: an aside grows to take in a same-font line
	// within this many line heights above or below it.
	asideAdjacencyRatio = 2.0
	// asideMinOverlap is the share of a line's width that must lie within
	// a note's x-range for the line to continue that note.
	asideMinOverlap = 0.5
)

// separateAsides pulls margin notes out of one column's top-to-bottom lines.
// A line sitting beside another line (overlapping vertically, apart
// horizontally) is a note when its font family differs from the column's
// dominant one; each note grows to take in the adjacent lines in its font
// that continue it, which catches its lines that happen not to
// sit beside a body line. Notes come back as vertically contiguous groups.
func separateAsides(lines []pdfLine) ([]pdfLine, [][]pdfLine) {
	if len(lines) < 2 {
		return lines, nil
	}
	dominant := dominantFamily(lines)
	families := make([]string, len(lines))
	for i, l := range lines {
		families[i] = dominantFamily([]pdfLine{l})
	}

	aside := make([]bool, len(lines))
	found := false
	for i := range lines {
		for j := range lines {
			if i != j && families[i] != dominant && sideBySide(lines[i], lines[j]) {
				aside[i] = true
				found = true
			}
		}
	}
	if !found {
		return lines, nil
	}
	growAsides(lines, families, aside)

	var body, notes []pdfLine
	for i, l := range lines {
		if aside[i] {
			notes = append(notes, l)
		} else {
			body = append(body, l)
		}
	}
	return body, clusterAsides(notes)
}

// growAsides marks every line that shares an aside's font family and
// continues it (withinAside), until nothing changes.
func growAsides(lines []pdfLine, families []string, aside []bool) {
	for changed := true; changed; {
		changed = false
		for i, l := range lines {
			if aside[i] {
				continue
			}
			for j, a := range lines {
				if aside[j] && families[j] == families[i] && withinAside(l, a) {
					aside[i] = true
					changed = true
					break
				}
			}
		}
	}
}

// withinAside reports whether l continues note line a: most of l lies in
// a's x-range (a ragged-right note varies in width; a full-width body line
// overlaps it only in part) and it sits within asideAdjacencyRatio line
// heights of a.
func withinAside(l, a pdfLine) bool {
	overlap := min(l.right, a.right) - max(l.left, a.left)
	if overlap < asideMinOverlap*(l.right-l.left) {
		return false
	}
	reach := asideAdjacencyRatio * max(l.top-l.bottom, a.top-a.bottom)
	return l.bottom-a.top <= reach && a.bottom-l.top <= reach
}

// sideBySide reports whether two lines overlap vertically but not
// horizontally.
func sideBySide(a, b pdfLine) bool {
	if a.bottom >= b.top || b.bottom >= a.top {
		return false
	}
	return a.right < b.left || b.right < a.left
}

// dominantFamily is the font family covering the most characters.
func dominantFamily(lines []pdfLine) string {
	counts := map[string]int{}
	best, bestCount := "", 0
	for _, l := range lines {
		for _, c := range l.chars {
			f := fontFamily(c.font)
			counts[f]++
			if counts[f] > bestCount || (counts[f] == bestCount && f < best) {
				best, bestCount = f, counts[f]
			}
		}
	}
	return best
}

// clusterAsides groups note lines, top to bottom, into contiguous notes.
func clusterAsides(lines []pdfLine) [][]pdfLine {
	sortLinesTopToBottom(lines)
	var groups [][]pdfLine
	for _, l := range lines {
		n := len(groups)
		if n > 0 && withinAside(l, groups[n-1][len(groups[n-1])-1]) {
			groups[n-1] = append(groups[n-1], l)
			continue
		}
		groups = append(groups, []pdfLine{l})
	}
	for _, g := range groups {
		applyColStats(g, g[0].col)
	}
	return groups
}

// insertAsides places each note into a column's stream right after the
// last item above the note's bottom, i.e. beside the text it sits next to.
func insertAsides(items []streamItem, notes [][]pdfLine) []streamItem {
	sort.SliceStable(notes, func(i, j int) bool {
		return notes[i][0].yMid() > notes[j][0].yMid()
	})
	for _, note := range notes {
		bottom := note[len(note)-1].bottom
		at := 0
		for i, it := range items {
			if it.line != nil && it.line.yMid() > bottom {
				at = i + 1
			}
		}
		//nolint:exhaustruct // streamItem is a line/figure/aside union
		item := streamItem{aside: note}
		items = append(items[:at], append([]streamItem{item}, items[at:]...)...)
	}
	return items
}
