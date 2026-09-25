package services

import (
	"sort"
	"strings"
	"unicode"

	"github.com/klippa-app/go-pdfium/responses"
)

// pdfChar is one character box in PDF point space (origin bottom-left, y
// increases upward), as reported by GetPageTextStructured.
type pdfChar struct {
	text                     string
	left, top, right, bottom float64
	// font is the name of the font this character was rendered with (empty
	// when font information wasn't collected). A change in font name between
	// two adjacent characters marks a text-run boundary — see buildLine.
	font string
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
	// chars is the line's own character boxes, retained so its text can be
	// re-joined with a different space-gap threshold once the document-wide
	// modal character height is known — see rebuildHeadingLineText (issue
	// #1698: tracked small-caps display headings letter-space wide enough
	// to clear the body-text threshold).
	chars []pdfChar
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
		var font string
		if c.FontInformation != nil {
			font = c.FontInformation.Name
		}
		chars = append(chars, pdfChar{
			text:   text,
			left:   c.PointPosition.Left,
			top:    c.PointPosition.Top,
			right:  c.PointPosition.Right,
			bottom: c.PointPosition.Bottom,
			font:   font,
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
	// headingSpaceRatio is the space-gap threshold used when re-joining
	// heading-sized lines: tracked display headings letter-space up to
	// ~0.28 * their line height while their word gaps start at ~0.65, so
	// only body text's tighter 0.25 threshold mis-splits their words
	// (issue #1698).
	headingSpaceRatio = 0.45
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

// clusterNormalCharsOverlapMarginRatio is the small float-rounding tolerance
// applied to the vertical-interval overlap check in clusterNormalChars — not
// a line-spacing allowance like lineGroupYMidRatio (that ratio is deliberately
// too large for this check: applying it here reintroduces the false merge it
// was tuned to avoid, see the comment below).
const clusterNormalCharsOverlapMarginRatio = 0.05

// clusterNormalChars groups normal-height characters into lines by
// vertical-interval overlap rather than y-midpoint distance: sort by
// descending midpoint, then join a character to the line being built when
// its own [bottom, top] box overlaps the running envelope (min bottom, max
// top seen so far) of the line, within a small float-rounding margin.
//
// A single physical line set in a large or stylized font can span a wide
// range of y-midpoints — a cap-height letter's midpoint sits well above an
// x-height letter's, which sits above a descender's — and that spread can
// exceed lineGroupYMidRatio * medH when medH is calibrated off a much
// smaller body-text font sharing the page (the chapter-title fracturing in
// issue #1651). Cap, x-height, and descender glyphs on one baseline all
// still overlap each other's box near the baseline/x-height band regardless
// of font size, so overlap keeps them together where midpoint distance
// would not. Genuinely separate lines set with normal leading still don't
// overlap, so they still split — this only requires actual box overlap, not
// lineGroupYMidRatio * medH of slack the way attachSmallChars allows for
// small punctuation: that much tolerance here would let lines separated by
// a narrow but real gap merge into one.
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

// lastRune/firstRune return the last/first rune of s, or the zero rune for
// an empty string.
func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// needsRunBoundarySpace reports whether a space belongs between two
// horizontally-adjacent characters already known to sit on the same line:
// either the physical gap between their boxes exceeds spaceRatio * medH
// (body text uses lineSpaceGapRatio; heading-sized lines are re-joined
// with headingSpaceRatio — see rebuildHeadingLineText), or they come from
// different font/style runs (prev.font != c.font, both known) and both
// sides of the boundary are alphabetic. The latter catches #1653: PDFium
// reports no whitespace glyph at a run boundary that falls mid-word-gap
// (e.g. "of" in one run, "Growth" in an adjacent italic run), so the
// geometric gap alone can be far too small to notice — but a run boundary
// between two letters reliably is a word boundary in body text. Font
// information is unset (empty string on both sides) unless
// CollectFontInformation was requested, so this never fires without it —
// buildLine falls back to the pre-#1653 gap-only behavior.
func needsRunBoundarySpace(prev, c pdfChar, medH, spaceRatio float64) bool {
	gap := c.left - prev.right
	if gap > spaceRatio*medH {
		return true
	}
	// A heading line re-joined with the raised ratio still spaces the
	// sub-band between the two ratios — every real word space observed in
	// tracked display headings sits there except one shape: a full-height
	// capital immediately followed by a much shorter lowercase letter is
	// the line's small-caps initial continuing its own word ("I" +
	// "ntroduction"), not a word boundary (#1698).
	if gap > lineSpaceGapRatio*medH && !isSmallCapsInitial(prev, c) {
		return true
	}
	if prev.font == "" || c.font == "" || prev.font == c.font {
		return false
	}
	return unicode.IsLetter(lastRune(prev.text)) && unicode.IsLetter(firstRune(c.text))
}

// smallCapsInitialRatio is how much taller than the following character a
// heading capital must be for the pair to read as a small-caps word's
// full-height initial rather than a word boundary: title-case lines space
// between same-height capitals and lowercase letters, while a tracked
// small-caps word's initial is far taller than its x-height continuation.
const smallCapsInitialRatio = 0.3

// isSmallCapsInitial reports whether prev is a single uppercase character
// rendered significantly taller than the lowercase character c — the
// signature of a small-caps word's initial, never of a word boundary.
func isSmallCapsInitial(prev, c pdfChar) bool {
	if prev.text != strings.ToUpper(prev.text) || len([]rune(prev.text)) != 1 {
		return false
	}
	if !unicode.IsUpper(firstRune(prev.text)) || !unicode.IsLower(firstRune(c.text)) {
		return false
	}
	prevH, curH := prev.top-prev.bottom, c.top-c.bottom
	if prevH <= 0 || curH <= 0 {
		return false
	}
	return prevH-curH > smallCapsInitialRatio*prevH
}

// buildLine sorts a line's characters left-to-right, joins their text
// (inserting a space where the horizontal gap between consecutive character
// boxes exceeds 0.25 * the line's median character height, or where two
// adjacent alphabetic characters come from different font/style runs — see
// #1653: a style change at a word boundary doesn't reliably line up with a
// physical gap large enough for the geometric check alone to catch), and
// computes the line's bounding box.
func buildLine(chars []pdfChar) pdfLine {
	sort.SliceStable(
		chars,
		func(i, j int) bool { return chars[i].left < chars[j].left },
	)

	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	left, right := chars[0].left, chars[0].right
	top, bottom := chars[0].top, chars[0].bottom

	for _, c := range chars {
		left = min(left, c.left)
		right = max(right, c.right)
		top = max(top, c.top)
		bottom = min(bottom, c.bottom)
	}

	// colRightEdge/colModalXStart/col are set later by assignColumns.
	return pdfLine{ //nolint:exhaustruct // set later by assignColumns
		text:             joinChars(chars, lineSpaceGapRatio),
		left:             left,
		top:              top,
		right:            right,
		bottom:           bottom,
		medianCharHeight: medH,
		chars:            chars,
	}
}

// joinChars builds a line's text from its character boxes, inserting a
// space between consecutive characters whose gap exceeds spaceRatio * the
// line's median character height, or whose font runs differ across an
// alphabetic boundary (the #1653 run check, always applied).
func joinChars(chars []pdfChar, spaceRatio float64) string {
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	var b strings.Builder
	for i, c := range chars {
		if i > 0 && needsRunBoundarySpace(chars[i-1], c, medH, spaceRatio) {
			b.WriteByte(' ')
		}
		b.WriteString(c.text)
	}
	return b.String()
}

// rebuildHeadingLineText re-joins the text of every heading-sized line
// (character height at least headingH1Ratio * the document's modal body-text
// height) with headingSpaceRatio as the space-gap threshold, in place.
//
// Tracked display headings letter-space their glyphs: in the reference book,
// a small-caps heading's inter-letter gaps reach 0.28 * the line's own
// character height — above the body-text threshold of 0.25 — while its word
// gaps start at 0.65, so the body-text ratio splits words like
// "I ntroduction" and "N otes" (#1698). Body-sized lines keep the body-text
// threshold: there, word gaps dip as low as 0.12 * line height, inside the
// letter-gap range, so only the cleanly-separable heading band gets the
// higher ratio.
func rebuildHeadingLineText(pages []pageResult, docModalHeight float64) {
	if docModalHeight <= 0 {
		return
	}
	threshold := headingH1Ratio * docModalHeight
	for _, p := range pages {
		for _, item := range p.items {
			if item.line == nil || len(item.line.chars) < 2 {
				continue
			}
			if item.line.medianCharHeight < threshold {
				continue
			}
			item.line.text = joinChars(item.line.chars, headingSpaceRatio)
		}
	}
}
