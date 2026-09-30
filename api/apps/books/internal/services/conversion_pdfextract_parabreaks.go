package services

import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// pageRefEndRe matches a line ending in an index page reference ("193",
// "131–135", "ix").
var pageRefEndRe = regexp.MustCompile(`(?:\d+(?:[–-]\d+)?|\b[ivxlc]+)\.?$`)

// Step 4 (paragraphs) thresholds, in median character widths unless noted.
const (
	// paragraphGapRatio: a vertical gap above this many line heights.
	paragraphGapRatio = 1.5
	// paragraphIndentChars/paragraphMaxIndentChars bound a first-line
	// indent: a bigger step right is text wrapping around a margin note.
	paragraphIndentChars    = 2
	paragraphMaxIndentChars = 6
	// hangingIndentChars bounds a hanging continuation's step right.
	hangingIndentChars = 3
	// alignedChars: lines starting within this distance share a margin.
	alignedChars = 0.5
	// paragraphShortLineRatio: a line ending short of the margin by more
	// than this fraction ends its paragraph.
	paragraphShortLineRatio = 0.85
	// justifiedEdgeChars: lines ending within this many characters of each
	// other are justified to a shared margin.
	justifiedEdgeChars = 0.25
	// minIndexLines/minIndexShare: a column is an index when at least this
	// many lines, and this share of them, end in a page reference.
	minIndexLines = 5
	minIndexShare = 0.5
	// minHangingSteps/minHangingShare: a column reads as a hanging-indent
	// layout with at least this many hanging continuations, making up at
	// least this share of its lines.
	minHangingSteps = 3
	minHangingShare = 0.2
	// localEdgeWindow/localEdgeAlignChars define a line's block for its
	// right margin: lines this many above and below that start within this
	// many characters of it.
	localEdgeWindow     = 3
	localEdgeAlignChars = 2.5
)

// paraRules carries the page's typographic stats into paragraph breaking.
type paraRules struct {
	medLineHeight, medCharWidth float64
	// hanging marks a column set in hanging indents (a bibliography, a
	// glossary): entries start at the margin, continuation lines step
	// right.
	hanging bool
	// index marks a column of index entries (see indexColumns).
	index bool
}

// startsNewParagraph implements step 4, judging cur against para, the lines
// so far: a paragraph breaks at a gap, a list marker, a return to a hanging
// entry's margin, an index line after a page reference, a first-line indent
// (a moderate step right of the previous line, not under a marker's text or
// continuing a hanging entry), or a short line not justified with the one
// above it.
func startsNewParagraph(para []pdfLine, cur pdfLine, r paraRules) bool {
	prev := para[len(para)-1]
	if r.medLineHeight > 0 && prev.bottom-cur.top > paragraphGapRatio*r.medLineHeight {
		return true
	}
	if listMarkerRe.MatchString(cur.text) {
		return true
	}
	ch := r.medCharWidth
	if ch > 0 {
		if returnsToEntryMargin(para, cur, ch) {
			return true
		}
		step := cur.left - prev.left
		if r.index && step <= alignedChars*ch &&
			pageRefEndRe.MatchString(strings.TrimSpace(prev.text)) {
			return true
		}
		textStart, marked := markerTextStart(prev)
		switch {
		case marked && math.Abs(cur.left-textStart) <= ch:
			// A hanging continuation under a list marker.
		case r.hanging && step > alignedChars*ch && step <= hangingIndentChars*ch &&
			!endsShort(prev, cur, ch):
			return false
		case step > paragraphIndentChars*ch && step <= paragraphMaxIndentChars*ch:
			return true
		}
	}
	// A line ending where the line above it ends is justified to a shared
	// margin — narrowed beside a figure, it's still a full line.
	if len(para) > 1 &&
		math.Abs(para[len(para)-2].right-prev.right) <= justifiedEdgeChars*ch {
		return false
	}
	return endsShort(prev, cur, ch)
}

// returnsToEntryMargin reports whether para has a hanging shape (its second
// line steps a little right of its first) and cur is back at the first
// line's margin.
func returnsToEntryMargin(para []pdfLine, cur pdfLine, ch float64) bool {
	if len(para) <= 1 {
		return false
	}
	hang := para[1].left - para[0].left
	return hang > alignedChars*ch && hang <= hangingIndentChars*ch &&
		cur.left <= para[0].left+alignedChars*ch
}

// hangingColumns reports, per column, whether its lines read as a
// hanging-indent layout: continuation lines stepping right after full
// lines outnumber first-line indents (an indent after a short line,
// followed by a return to the previous margin) and make up a fair share of
// its lines. Lines after a list marker
// don't count, since markers are handled on their own.
func hangingColumns(items []streamItem, ch float64) [2]bool {
	var hang, indent [2]int
	var cols [2][]pdfLine
	for _, it := range items {
		if it.line != nil {
			cols[it.line.col] = append(cols[it.line.col], *it.line)
		}
	}
	for c, lines := range cols {
		for i := 1; i < len(lines); i++ {
			prev, cur := lines[i-1], lines[i]
			step := cur.left - prev.left
			if step <= alignedChars*ch {
				continue
			}
			short := endsShort(prev, cur, ch)
			if !short && step <= hangingIndentChars*ch &&
				!listMarkerRe.MatchString(prev.text) {
				hang[c]++
			}
			if short && step <= paragraphMaxIndentChars*ch && i+1 < len(lines) &&
				lines[i+1].left <= prev.left+alignedChars*ch {
				indent[c]++
			}
		}
	}
	var hanging [2]bool
	for c := range cols {
		hanging[c] = hang[c] >= minHangingSteps && hang[c] > indent[c] &&
			float64(hang[c]) >= minHangingShare*float64(len(cols[c]))
	}
	return hanging
}

// endsShort reports whether prev ends before its margin: the first word of
// cur would have fitted after it, or it stops well short of the margin and
// of cur. The margin is prev's block's right edge, or — when prev and cur
// end at the same x, justified to a margin narrowed beside a figure — that
// x.
func endsShort(prev, cur pdfLine, medCharWidth float64) bool {
	edge := prev.colRightEdge
	if prev.localRightEdge > 0 {
		edge = prev.localRightEdge
	}
	if math.Abs(prev.right-cur.right) <= justifiedEdgeChars*medCharWidth {
		edge = max(prev.right, cur.right)
	}
	if edge <= 0 {
		return false
	}
	if prev.right+medCharWidth+firstWordWidth(cur) <= edge {
		return true
	}
	return prev.right < paragraphShortLineRatio*edge &&
		prev.right < cur.right-medCharWidth
}

// firstWordWidth is the width of a line's first word.
func firstWordWidth(l pdfLine) float64 {
	word, _, _ := strings.Cut(l.text, " ")
	n := utf8.RuneCountInString(word)
	if n == 0 || n > len(l.chars) {
		return 0
	}
	return l.chars[n-1].right - l.left
}

// setLocalRightEdges sets each line's localRightEdge to the widest line
// among its neighbours in the same block: a ragged-right box set narrower
// than the column has its own margin, and measuring its lines against the
// column's would end every one of them short.
func setLocalRightEdges(items []streamItem, ch float64) {
	var cols [2][]*pdfLine
	for _, it := range items {
		if it.line != nil {
			cols[it.line.col] = append(cols[it.line.col], it.line)
		}
	}
	for _, lines := range cols {
		for i, l := range lines {
			edge, aligned := l.right, false
			for j := max(0, i-localEdgeWindow); j <= min(len(lines)-1, i+localEdgeWindow); j++ {
				if j != i && math.Abs(lines[j].left-l.left) <= localEdgeAlignChars*ch {
					edge, aligned = max(edge, lines[j].right), true
				}
			}
			// A line aligned with nothing (a centred heading) has no block
			// of its own; it keeps the column's margin.
			if aligned {
				l.localRightEdge = edge
			}
		}
	}
}

// indexColumns reports, per column, whether most of its lines end in a page
// reference — an index, whose entries rarely fill a line, so a line ending
// before the margin says nothing about where its entry ends.
func indexColumns(items []streamItem) [2]bool {
	var refs, total [2]int
	for _, it := range items {
		if it.line == nil {
			continue
		}
		total[it.line.col]++
		if pageRefEndRe.MatchString(strings.TrimSpace(it.line.text)) {
			refs[it.line.col]++
		}
	}
	var index [2]bool
	for c := range index {
		index[c] = refs[c] >= minIndexLines &&
			float64(refs[c]) >= minIndexShare*float64(total[c])
	}
	return index
}
