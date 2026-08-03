package services

import (
	"sort"
	"strings"

	"github.com/klippa-app/go-pdfium/responses"
)

// pdfChar is one character box in PDF point space (origin bottom-left, y
// increases upward), as reported by GetPageTextStructured.
type pdfChar struct {
	text                     string
	left, top, right, bottom float64
}

// pdfLine is one reconstructed line of text with its bounding box and the
// per-column typographic stats needed for paragraph-break detection.
type pdfLine struct {
	text                     string
	left, top, right, bottom float64
	medianCharHeight         float64
	colRightEdge             float64
	colModalXStart           float64
	// col distinguishes the left/single column (0) from the right column (1)
	// so paragraph grouping can force a break at the column boundary instead
	// of continuing to compare gap/indent against a line from another column.
	col int
}

//nolint:mnd // midpoint of a bounding box
func (l pdfLine) yMid() float64 { return (l.top + l.bottom) / 2 }

//nolint:mnd // midpoint of a bounding box
func (c pdfChar) yMid() float64 { return (c.top + c.bottom) / 2 }

//nolint:mnd // midpoint of a bounding box
func (l pdfLine) xMid() float64 { return (l.left + l.right) / 2 }

// pdfiumSoftHyphenMarker is the Unicode value (U+0002) PDFium's text-page
// builder substitutes for a hyphen it has itself detected at the end of one
// text-showing run immediately followed by continuing text — an artifact of
// the dehyphenation PDFium's own plain-text extraction performs internally,
// which leaks into the per-character Unicode reported here. The glyph
// actually rendered is a normal hyphen, so it's mapped back to "-" to let our
// own hyphenation join (step 5) decide, rather than losing the character.
const pdfiumSoftHyphenMarker = "\x02"

// extractChars converts a structured-text response into pdfChars, dropping
// control characters (empty text) and whitespace — line/paragraph spacing is
// reconstructed from geometry, not from the whitespace glyphs PDFium reports.
func extractChars(resp *responses.GetPageTextStructured) []pdfChar {
	chars := make([]pdfChar, 0, len(resp.Chars))
	for _, c := range resp.Chars {
		text := c.Text
		if text == pdfiumSoftHyphenMarker {
			text = "-"
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		chars = append(chars, pdfChar{
			text:   text,
			left:   c.PointPosition.Left,
			top:    c.PointPosition.Top,
			right:  c.PointPosition.Right,
			bottom: c.PointPosition.Bottom,
		})
	}
	return chars
}

// lineGroupYMidRatio/lineSpaceGapRatio implement step 1 (lines): characters
// join a line when their y-midpoint is within this fraction of the page's
// median character height of the line's running midpoint, and a space is
// inserted within a line when the horizontal gap between consecutive
// character boxes exceeds this fraction of the line's median character
// height.
const (
	lineGroupYMidRatio = 0.5
	lineSpaceGapRatio  = 0.25
)

// median returns the middle value of a sorted-in-place copy of vs, or 0 for
// an empty input.
func median(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	sorted := make([]float64, len(vs))
	copy(sorted, vs)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

func medianCharHeight(chars []pdfChar) float64 {
	heights := make([]float64, len(chars))
	for i, c := range chars {
		heights[i] = c.top - c.bottom
	}
	return median(heights)
}

func medianCharWidth(chars []pdfChar) float64 {
	widths := make([]float64, len(chars))
	for i, c := range chars {
		widths[i] = c.right - c.left
	}
	return median(widths)
}

// normalCharHeightRatio distinguishes an ordinary letter — whose box spans
// most of the page's median character height — from small punctuation
// (commas, apostrophes, quotation marks, ...) whose box is much shorter and
// sits off to one side of the baseline: low for commas/descenders, high for
// apostrophes/quotes. Only normal-height characters take part in line
// clustering (clusterNormalChars); small characters are attached afterwards
// to whichever established line they're vertically closest to
// (attachSmallChars). Letting punctuation's own off-center box influence
// clustering is what caused #594 (commas split into their own line) and
// #618 (apostrophes/quotes did the same, the opposite direction) — and
// widening the clustering window to tolerate both directions at once (tried
// while fixing #618) let unrelated lines merge into one, scrambling reading
// order within the merged group.
const normalCharHeightRatio = 0.7

// groupLines clusters chars into lines (step 1 of the text algorithm): the
// normal-height characters are clustered by y-midpoint proximity first
// (clusterNormalChars), then every short-box punctuation character is
// attached to its nearest resulting line (attachSmallChars) rather than
// being allowed to shift where lines split.
func groupLines(chars []pdfChar) []pdfLine {
	if len(chars) == 0 {
		return nil
	}
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	var normal, small []pdfChar
	for _, c := range chars {
		if c.top-c.bottom >= normalCharHeightRatio*medH {
			normal = append(normal, c)
		} else {
			small = append(small, c)
		}
	}
	if len(normal) == 0 {
		// Degenerate page (every character is "small") — cluster everything
		// so a page like this still produces output.
		normal, small = chars, nil
	}

	groups := clusterNormalChars(normal, medH)
	groups = attachSmallChars(groups, small, medH)

	// clusterNormalChars produces groups in descending y-midpoint (top to
	// bottom) order; attachSmallChars can append a new group past the end
	// when no existing line is close enough, so restore that order before
	// building lines.
	sort.SliceStable(groups, func(i, j int) bool {
		return groupYMid(groups[i]) > groupYMid(groups[j])
	})

	lines := make([]pdfLine, len(groups))
	for i, g := range groups {
		lines[i] = buildLine(g)
	}
	return lines
}

// clusterNormalChars groups normal-height characters into lines by
// y-midpoint proximity: sort by descending midpoint, then join a character
// to the line being built when its midpoint is within lineGroupYMidRatio *
// medH of that line's running average midpoint.
func clusterNormalChars(chars []pdfChar, medH float64) [][]pdfChar {
	sorted := make([]pdfChar, len(chars))
	copy(sorted, chars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].yMid() > sorted[j].yMid()
	})

	var groups [][]pdfChar
	var group []pdfChar
	var midSum float64
	threshold := lineGroupYMidRatio * medH

	flush := func() {
		if len(group) == 0 {
			return
		}
		groups = append(groups, group)
		group = nil
		midSum = 0
	}

	for _, c := range sorted {
		mid := c.yMid()
		if len(group) > 0 {
			avg := midSum / float64(len(group))
			if diff := avg - mid; diff > threshold || diff < -threshold {
				flush()
			}
		}
		group = append(group, c)
		midSum += mid
	}
	flush()

	return groups
}

// attachSmallChars assigns each short-box character to a group by
// vertical-interval overlap, not by distance from a single point: a small
// character joins the group whose [bottom, top] envelope — computed once
// from that group's normal-height characters, before any small characters
// are attached, so it can't grow across attachments — overlaps the
// character's own [bottom, top] box within lineGroupYMidRatio * medH.
// Overlap tolerates punctuation sitting on either side of the baseline
// (comma low, apostrophe high); among multiple overlapping groups the one
// whose y-midpoint is closest wins. A small character with no overlapping
// group starts its own group, so a page of pure punctuation still produces
// output.
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

// buildLine sorts a line's characters left-to-right, joins their text
// (inserting a space where the horizontal gap between consecutive character
// boxes exceeds 0.25 * the line's median character height), and computes the
// line's bounding box.
func buildLine(chars []pdfChar) pdfLine {
	sort.SliceStable(
		chars,
		func(i, j int) bool { return chars[i].left < chars[j].left },
	)

	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	var b strings.Builder
	left, right := chars[0].left, chars[0].right
	top, bottom := chars[0].top, chars[0].bottom

	for i, c := range chars {
		if i > 0 && c.left-chars[i-1].right > lineSpaceGapRatio*medH {
			b.WriteByte(' ')
		}
		b.WriteString(c.text)

		left = min(left, c.left)
		right = max(right, c.right)
		top = max(top, c.top)
		bottom = min(bottom, c.bottom)
	}

	// colRightEdge/colModalXStart/col are set later by assignColumns.
	return pdfLine{ //nolint:exhaustruct // set later by assignColumns
		text:             b.String(),
		left:             left,
		top:              top,
		right:            right,
		bottom:           bottom,
		medianCharHeight: medH,
	}
}
