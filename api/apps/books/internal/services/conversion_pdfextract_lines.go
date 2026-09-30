package services

import (
	"sort"
)

// normalCharHeightRatio separates ordinary letters from small punctuation
// (commas low, apostrophes and quotes high), whose off-centre boxes would
// otherwise split lines or merge neighbouring ones: only normal-height
// characters cluster into lines, small ones attach afterwards.
const normalCharHeightRatio = 0.7

// groupLines clusters chars into lines (step 1 of the text algorithm): runs
// PDFium streamed together join lines by baseline (a margin note on another
// leading never chains into the body beside it); the remaining characters
// attach to an overlapping unit line, else cluster by overlap
// (clusterNormalChars), with small punctuation attached last.
func groupLines(chars []pdfChar) []pdfLine {
	if len(chars) == 0 {
		return nil
	}
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	units, loose := streamUnits(chars)
	groups := mergeUnitsByBaseline(units, medH)

	var normal, small []pdfChar
	for _, c := range loose {
		if c.top-c.bottom >= normalCharHeightRatio*medH {
			normal = append(normal, c)
		} else {
			small = append(small, c)
		}
	}
	if len(normal) == 0 && len(groups) == 0 {
		// Degenerate page (every character is "small") — cluster everything
		// so a page like this still produces output.
		normal, small = loose, nil
	}

	groups, normal = attachToUnitLines(groups, normal, medH)
	groups = append(groups, clusterNormalChars(normal, medH)...)
	groups = attachSmallChars(groups, small, medH)

	// Groups must run top to bottom before building lines; unit lines,
	// clustered lines, and attachSmallChars's new groups each come from
	// their own pass.
	sort.SliceStable(groups, func(i, j int) bool {
		return groupYMid(groups[i]) > groupYMid(groups[j])
	})

	lines := make([]pdfLine, len(groups))
	for i, g := range groups {
		lines[i] = buildLine(g)
	}
	return lines
}

// clusterNormalCharsOverlapMarginRatio is the small float-rounding tolerance
// applied to the vertical-interval overlap check in clusterNormalChars — not
// a line-spacing allowance like lineGroupYMidRatio (that ratio is deliberately
// too large for this check: applying it here reintroduces the false merge it
// was tuned to avoid, see the comment below).
const clusterNormalCharsOverlapMarginRatio = 0.05

// clusterNormalChars groups normal-height characters into lines by
// vertical-interval overlap with the line's running envelope (within a
// float-rounding margin), not y-midpoint distance: a large title's cap,
// x-height and descender glyphs have spread-out midpoints but overlapping
// boxes, while separate lines with normal leading don't overlap.
func clusterNormalChars(chars []pdfChar, medH float64) [][]pdfChar {
	sorted := make([]pdfChar, len(chars))
	copy(sorted, chars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].yMid() > sorted[j].yMid()
	})

	var groups [][]pdfChar
	var group []pdfChar
	var envTop, envBottom float64
	margin := clusterNormalCharsOverlapMarginRatio * medH

	flush := func() {
		if len(group) == 0 {
			return
		}
		groups = append(groups, group)
		group = nil
	}

	for _, c := range sorted {
		if len(group) > 0 && (c.bottom > envTop+margin || c.top < envBottom-margin) {
			flush()
		}
		if len(group) == 0 {
			envTop, envBottom = c.top, c.bottom
		} else {
			envTop = max(envTop, c.top)
			envBottom = min(envBottom, c.bottom)
		}
		group = append(group, c)
	}
	flush()

	return groups
}

// attachSmallChars adds each small character to the group whose envelope,
// fixed from its normal-height characters, overlaps it within
// lineGroupYMidRatio * medH, the closest y-midpoint winning; one with no
// overlapping group starts its own, so a page of punctuation still reads.
func attachSmallChars(groups [][]pdfChar, small []pdfChar, medH float64) [][]pdfChar {
	margin := lineGroupYMidRatio * medH

	envelopes := make([]pdfLine, len(groups))
	for i, g := range groups {
		envelopes[i] = buildLine(append([]pdfChar(nil), g...))
	}

	for _, c := range small {
		best := -1
		var bestDist float64

		for i, env := range envelopes {
			if c.bottom > env.top+margin || c.top < env.bottom-margin {
				continue
			}
			dist := groupYMid(groups[i]) - c.yMid()
			if dist < 0 {
				dist = -dist
			}
			if best == -1 || dist < bestDist {
				best = i
				bestDist = dist
			}
		}

		if best == -1 {
			groups = append(groups, []pdfChar{c})
			continue
		}
		groups[best] = append(groups[best], c)
	}

	return groups
}

// groupYMid returns the average y-midpoint of a group of characters.
func groupYMid(g []pdfChar) float64 {
	var sum float64
	for _, c := range g {
		sum += c.yMid()
	}
	return sum / float64(len(g))
}
