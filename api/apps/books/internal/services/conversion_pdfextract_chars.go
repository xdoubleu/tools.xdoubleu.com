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
	font   string
	stream streamPos
}

// streamPos is a character's place in PDFium's text stream. PDFium reports
// word spaces as whitespace characters (real or generated from the font's
// advance widths), which is a far better word-boundary signal than the gap
// between tight glyph boxes: side-bearings split words like "genera l ly",
// and an overhanging "f" hides the gap in "of the".
type streamPos struct {
	// seq is the 1-based position among the page's non-whitespace
	// characters; 0 means unknown (synthetic characters).
	seq int
	// spaceBefore/breakBefore record whitespace or a PDFium line break
	// between this character and the previous non-whitespace one.
	spaceBefore bool
	breakBefore bool
	// afterLigature marks a space right after a ligature. InDesign emits
	// one after every ligature, mid-word included ("fl ows"), so it only
	// counts when the gap to this character is word-sized.
	afterLigature bool
	// generatedSpace marks a zero-width space PDFium inferred rather than
	// read from the stream; it guesses spaces between a tracked title's
	// letters too, so only a gap past the line's threshold confirms it.
	generatedSpace bool
}

// noStreamPos is a character with no known stream position.
//
//nolint:gochecknoglobals // deliberately the zero value; read-only
var noStreamPos streamPos

// pdfLine is one reconstructed line of text with its bounding box and the
// per-column typographic stats needed for paragraph-break detection.
type pdfLine struct {
	text                     string
	left, top, right, bottom float64
	medianCharHeight         float64
	colRightEdge             float64
	colModalXStart           float64
	// localRightEdge is the right margin of the line's own block (see
	// setLocalRightEdges); 0 until set.
	localRightEdge float64
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
// control characters (empty text) and whitespace. Whitespace survives only
// as each following character's streamPos flags.
func extractChars(resp *responses.GetPageTextStructured) []pdfChar {
	chars := make([]pdfChar, 0, len(resp.Chars))
	var pos streamPos
	for _, c := range resp.Chars {
		text := c.Text
		if text == pdfiumSoftHyphenMarker {
			text = "-"
		}
		if text == "" {
			continue
		}
		if strings.TrimSpace(text) == "" {
			switch {
			case strings.ContainsAny(text, "\r\n"):
				pos.breakBefore = true
			case c.PointPosition.Right <= c.PointPosition.Left:
				pos.generatedSpace = true
			default:
				pos.spaceBefore = true
				pos.afterLigature = endsInLigature(chars)
			}
			continue
		}
		var font string
		if c.FontInformation != nil {
			font = c.FontInformation.Name
		}
		pos.seq++
		chars = append(chars, pdfChar{
			text:   text,
			left:   c.PointPosition.Left,
			top:    c.PointPosition.Top,
			right:  c.PointPosition.Right,
			bottom: c.PointPosition.Bottom,
			font:   font,
			stream: pos,
		})
		pos = streamPos{
			seq: pos.seq, spaceBefore: false, breakBefore: false,
			afterLigature: false, generatedSpace: false,
		}
	}
	return chars
}

// endsInLigature reports whether the last two characters share one box:
// PDFium's decomposition of a ligature glyph ("fl") into its letters.
func endsInLigature(chars []pdfChar) bool {
	n := len(chars)
	if n < 2 {
		return false
	}
	a, b := chars[n-2], chars[n-1]
	return a.left == b.left && a.right == b.right &&
		unicode.IsLetter(lastRune(a.text)) && unicode.IsLetter(firstRune(b.text))
}

// streamAdjacent reports whether c directly follows prev in PDFium's stream
// on the same PDFium line, so c's spaceBefore flag describes the gap
// between them.
func streamAdjacent(prev, c pdfChar) bool {
	return prev.stream.seq > 0 && c.stream.seq == prev.stream.seq+1 &&
		!c.stream.breakBefore
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
	// streamWideGapRatio spaces stream-adjacent characters that PDFium gave
	// no whitespace between but that are positioned a word apart (a tab
	// stop); letter gaps, tracked headings included, stay under ~0.3.
	streamWideGapRatio = 0.6
	// ligatureSpaceGapRatio: after a ligature's spurious space glyph, the
	// next letter sits at most ~0.3 away mid-word and ~0.5 after a real
	// word space.
	ligatureSpaceGapRatio = 0.4
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
// horizontally-adjacent characters already known to sit on the same line.
// Stream-adjacent characters follow PDFium's whitespace (plus the font-run
// check below). Otherwise: either the physical gap between their boxes exceeds spaceRatio * medH
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
	if streamAdjacent(prev, c) {
		space := c.stream.spaceBefore &&
			(!c.stream.afterLigature || gap > ligatureSpaceGapRatio*medH)
		generated := c.stream.generatedSpace && gap > spaceRatio*medH
		return space || generated || gap > streamWideGapRatio*medH ||
			isFontRunBoundary(prev, c)
	}
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
	return isFontRunBoundary(prev, c)
}

// isFontRunBoundary reports whether two letters come from different known
// fonts — a word boundary in body text (#1653).
func isFontRunBoundary(prev, c pdfChar) bool {
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
