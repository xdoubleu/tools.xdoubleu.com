package services

import (
	"sort"
	"strings"
	"unicode"

	"github.com/klippa-app/go-pdfium/responses"
)

// pdfChar is one character box in PDF point space (origin bottom-left).
type pdfChar struct {
	text                     string
	left, top, right, bottom float64
	// font is the rendering font name (empty unless collected); a change marks a
	// text-run boundary.
	font string
}

// pdfLine is one reconstructed line with its bounding box and column stats.
type pdfLine struct {
	text                     string
	left, top, right, bottom float64
	medianCharHeight         float64
	colRightEdge             float64
	colModalXStart           float64
	// col is 0 for the left/single column, 1 for the right, so grouping can force
	// a break at the column boundary.
	col int
	// chars is kept so heading lines can be re-joined once the document's modal
	// height is known (see rebuildHeadingLineText).
	chars []pdfChar
}

//nolint:mnd // midpoint of a bounding box
func (l pdfLine) yMid() float64 { return (l.top + l.bottom) / 2 }

//nolint:mnd // midpoint of a bounding box
func (c pdfChar) yMid() float64 { return (c.top + c.bottom) / 2 }

//nolint:mnd // midpoint of a bounding box
func (l pdfLine) xMid() float64 { return (l.left + l.right) / 2 }

// pdfiumSoftHyphenMarker (U+0002) is what PDFium substitutes for a hyphen it
// detected at a run end. The rendered glyph is a normal hyphen, so it maps
// back to "-" and our own dehyphenation decides.
const pdfiumSoftHyphenMarker = "\x02"

// extractChars drops control and whitespace characters; spacing is rebuilt
// from geometry.
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

// Step 1 (lines): characters join a line within lineGroupYMidRatio of the
// median char height, and a space is inserted when the horizontal gap exceeds
// lineSpaceGapRatio of it.
const (
	lineGroupYMidRatio = 0.5
	lineSpaceGapRatio  = 0.25
	// headingSpaceRatio re-joins heading lines: tracked display headings
	// letter-space up to ~0.28x line height, with word gaps from ~0.65.
	headingSpaceRatio = 0.45
)

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

// normalCharHeightRatio separates letters from short punctuation boxes.
// Only normal-height chars drive line clustering; punctuation is attached
// afterwards (attachSmallChars), because its off-center boxes split commas or
// quotes into their own lines, and a wider window merged unrelated lines.
const normalCharHeightRatio = 0.7

// groupLines clusters normal-height chars into lines, then attaches short
// punctuation to its nearest line.
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
		// Every character is small: cluster everything so the page still has output.
		normal, small = chars, nil
	}

	groups := clusterNormalChars(normal, medH)
	groups = attachSmallChars(groups, small, medH)

	// attachSmallChars may append groups past the end; restore top-to-bottom order.
	sort.SliceStable(groups, func(i, j int) bool {
		return groupYMid(groups[i]) > groupYMid(groups[j])
	})

	lines := make([]pdfLine, len(groups))
	for i, g := range groups {
		lines[i] = buildLine(g)
	}
	return lines
}

// clusterNormalCharsOverlapMarginRatio is a float-rounding tolerance only;
// lineGroupYMidRatio's slack here would merge separate lines.
const clusterNormalCharsOverlapMarginRatio = 0.05

// clusterNormalChars groups characters by vertical box overlap with the
// line's running envelope rather than midpoint distance: a large or stylized
// line's cap, x-height and descender midpoints can spread wider than a
// body-calibrated threshold, but their boxes still overlap, while separate
// lines with normal leading don't.
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

// attachSmallChars attaches each short character to the group whose
// normal-char envelope overlaps its box within lineGroupYMidRatio*medH,
// preferring the closest midpoint; otherwise it starts its own group.
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

func groupYMid(g []pdfChar) float64 {
	var sum float64
	for _, c := range g {
		sum += c.yMid()
	}
	return sum / float64(len(g))
}

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

// needsRunBoundarySpace reports a space between same-line characters when the
// gap exceeds spaceRatio*medH, or when both are letters from different font
// runs: PDFium emits no whitespace at a run boundary mid-word-gap ("of"
// + italic "Growth"). The font check needs CollectFontInformation.
func needsRunBoundarySpace(prev, c pdfChar, medH, spaceRatio float64) bool {
	gap := c.left - prev.right
	if gap > spaceRatio*medH {
		return true
	}
	// Between the two ratios, a gap is a space unless it's a small-caps initial
	// continuing its word ("I" + "ntroduction").
	if gap > lineSpaceGapRatio*medH && !isSmallCapsInitial(prev, c) {
		return true
	}
	if prev.font == "" || c.font == "" || prev.font == c.font {
		return false
	}
	return unicode.IsLetter(lastRune(prev.text)) && unicode.IsLetter(firstRune(c.text))
}

// smallCapsInitialRatio is how much taller a capital must be than the next
// char to count as a small-caps initial rather than a word boundary.
const smallCapsInitialRatio = 0.3

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

// buildLine sorts chars left-to-right, joins their text and computes the
// bounding box.
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

// joinChars joins a line's chars, spacing per needsRunBoundarySpace.
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

// rebuildHeadingLineText re-joins heading-sized lines (>= headingH1Ratio x
// modal body height) with headingSpaceRatio, in place. Body lines keep the
// lower ratio because their word gaps overlap heading letter gaps.
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
