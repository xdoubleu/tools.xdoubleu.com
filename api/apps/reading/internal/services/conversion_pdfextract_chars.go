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

func (l pdfLine) yMid() float64 { return (l.top + l.bottom) / 2 }
func (l pdfLine) xMid() float64 { return (l.left + l.right) / 2 }

// pdfiumSoftHyphenMarker is the Unicode value (U+0002) PDFium's text-page
// builder substitutes for a hyphen it has itself detected at the end of one
// text-showing run immediately followed by continuing text — an artifact of
// the dehyphenation PDFium's own plain-text extraction performs internally,
// which leaks into the per-character Unicode reported here. The glyph
// actually rendered is a normal hyphen, so it's mapped back to "-" to let our
// own hyphenation join (step 5) decide, rather than losing the character.
const pdfiumSoftHyphenMarker = ""

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

// groupLines clusters chars into lines by y-midpoint proximity (step 1 of the
// text algorithm): sort characters by descending y, then join a character to
// the line being built when its midpoint is within 0.5 * the page's median
// character height of that line's running average midpoint.
func groupLines(chars []pdfChar) []pdfLine {
	if len(chars) == 0 {
		return nil
	}
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	sorted := make([]pdfChar, len(chars))
	copy(sorted, chars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].yMid() > sorted[j].yMid()
	})

	var lines []pdfLine
	var group []pdfChar
	var midSum float64

	flush := func() {
		if len(group) == 0 {
			return
		}
		lines = append(lines, buildLine(group))
		group = nil
		midSum = 0
	}

	for _, c := range sorted {
		mid := c.yMid()
		if len(group) > 0 {
			avg := midSum / float64(len(group))
			if diff := avg - mid; diff > 0.5*medH || diff < -0.5*medH {
				flush()
			}
		}
		group = append(group, c)
		midSum += mid
	}
	flush()

	return lines
}

func (c pdfChar) yMid() float64 { return (c.top + c.bottom) / 2 }

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
		if i > 0 && c.left-chars[i-1].right > 0.25*medH {
			b.WriteByte(' ')
		}
		b.WriteString(c.text)

		left = min(left, c.left)
		right = max(right, c.right)
		top = max(top, c.top)
		bottom = min(bottom, c.bottom)
	}

	return pdfLine{
		text:             b.String(),
		left:             left,
		top:              top,
		right:            right,
		bottom:           bottom,
		medianCharHeight: medH,
	}
}
