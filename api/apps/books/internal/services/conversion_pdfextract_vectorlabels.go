package services

import (
	"math"
	"regexp"
	"strings"
)

// mergePanels joins regions of one figure — a diagram above its graph,
// stacked panels — that lie within vectorRegionMergeGap with no untaken
// text line between them.
func mergePanels(regions []box, lines []pdfLine, taken []bool) []box {
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(regions) && !merged; i++ {
			for j := i + 1; j < len(regions) && !merged; j++ {
				u := regions[i].union(regions[j])
				if regions[i].distance(regions[j]) > vectorRegionMergeGap ||
					textWithin(u, lines, taken) {
					continue
				}
				regions[i] = u
				regions = append(regions[:j], regions[j+1:]...)
				merged = true
			}
		}
	}
	return regions
}

// endsParagraphAbove reports whether l sits right under an untaken text
// line, aligned with it: a wrapped caption's short last line, not a label.
func endsParagraphAbove(l pdfLine, lines []pdfLine, taken []bool) bool {
	h := l.top - l.bottom
	for i, above := range lines {
		if taken[i] || above.bottom <= l.top || above.bottom-l.top > h {
			continue
		}
		if math.Abs(above.left-l.left) <= h {
			return true
		}
	}
	return false
}

// textWithin reports whether an untaken line's centre lies in r.
func textWithin(r box, lines []pdfLine, taken []bool) bool {
	for i, l := range lines {
		if !taken[i] && r.contains(l.xMid(), l.yMid()) {
			return true
		}
	}
	return false
}

// claimFigureRegion takes the lines inside r and the label lines around it
// (marking them in taken) and returns the grown region, unless r encloses
// prose — a ruled box around text, not a figure.
func claimFigureRegion(r box, lines []pdfLine, taken []bool) (box, bool) {
	var inside []int
	for i, l := range lines {
		if taken[i] || !r.contains(l.xMid(), l.yMid()) {
			continue
		}
		if l.right-l.left >= vectorProseWidthShare*r.width() &&
			len(strings.Fields(l.text)) >= vectorProseMinWords && !isLabelLine(l) {
			return r, false
		}
		inside = append(inside, i)
	}
	for _, i := range inside {
		taken[i] = true
		r = r.union(lineBox(lines[i]))
	}
	for grown := true; grown; {
		r, grown = takeLabels(r, lines, taken)
	}
	return r, true
}

// takeLabels takes every label line within reach of r into it, reporting
// whether any was taken.
func takeLabels(r box, lines []pdfLine, taken []bool) (box, bool) {
	grown := false
	for i, l := range lines {
		if taken[i] || !isLabelLine(l) || endsParagraphAbove(l, lines, taken) {
			continue
		}
		reach := vectorLabelReach
		if l.bottom < r.top && l.top > r.bottom {
			// Beside the figure: a legend or axis title, never body text.
			reach = vectorSideLabelReach
		}
		if r.distance(lineBox(l)) <= reach*(l.top-l.bottom) {
			taken[i] = true
			r = r.union(lineBox(l))
			grown = true
		}
	}
	return r, grown
}

// tickValueRe matches an axis tick value ("0", "2.5", "100%", "$50").
var tickValueRe = regexp.MustCompile(`^[-+$]?\d+(?:[.,]\d+)?%?$`)

// isLabelLine reports whether l reads as figure labels: each run of it
// between wide gaps (labels sharing a baseline) is a few words, or a row of
// tick values of any length.
func isLabelLine(l pdfLine) bool {
	for _, run := range labelRuns(l) {
		words := strings.Fields(run)
		if len(words) <= vectorLabelMaxWords {
			continue
		}
		for _, w := range words {
			if !tickValueRe.MatchString(w) {
				return false
			}
		}
	}
	return true
}

// labelRuns splits a line's text at gaps wider than vectorLabelGapChars
// median character widths.
func labelRuns(l pdfLine) []string {
	if len(l.chars) == 0 {
		return []string{l.text}
	}
	gap := vectorLabelGapChars * medianCharWidth(l.chars)
	var runs []string
	start := 0
	for i := 1; i <= len(l.chars); i++ {
		if i == len(l.chars) || l.chars[i].left-l.chars[i-1].right > gap {
			runs = append(runs, joinChars(l.chars[start:i], lineSpaceGapRatio))
			start = i
		}
	}
	return runs
}

// charsOutsideLines returns chars minus those belonging to taken lines.
func charsOutsideLines(chars []pdfChar, lines []pdfLine, taken []bool) []pdfChar {
	type charKey struct {
		text         string
		left, bottom float64
	}
	drop := map[charKey]bool{}
	for i, l := range lines {
		if !taken[i] {
			continue
		}
		for _, c := range l.chars {
			drop[charKey{text: c.text, left: c.left, bottom: c.bottom}] = true
		}
	}
	kept := make([]pdfChar, 0, len(chars))
	for _, c := range chars {
		if !drop[charKey{text: c.text, left: c.left, bottom: c.bottom}] {
			kept = append(kept, c)
		}
	}
	return kept
}
