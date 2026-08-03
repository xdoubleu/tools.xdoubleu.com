package services

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// headingH1Ratio/headingH2Ratio implement step 6 (headings): a paragraph
// whose median character height exceeds these ratios of the document's modal
// (body text) character height becomes an <h1>/<h2>.
const (
	headingH1Ratio = 1.4
	headingH2Ratio = 1.15
	// paragraphGapRatio/paragraphIndentChars implement step 4 (paragraphs).
	paragraphGapRatio    = 1.5
	paragraphIndentChars = 2
	// paragraphShortLineRatio: a line ending short of the column's right
	// margin by more than this fraction ends its paragraph.
	paragraphShortLineRatio = 0.85
)

// streamItem is one entry in a page's reading-order stream: either a line of
// text or a figure placed between lines.
type streamItem struct {
	line   *pdfLine
	figure *pdfFigure
}

// htmlBlock is one block-level element of the extracted document: a
// paragraph, heading, or image.
type htmlBlock struct {
	html string
	tag  string // "p", "h1", "h2", or "img" — used for title fallback/tests.
	text string // raw (unescaped) text; empty for "img".
}

// mergeColumn interleaves a column's lines (already sorted top-to-bottom)
// with its figures, inserting each figure after the last line whose
// y-midpoint is above the figure's top bound (step: figure placement).
func mergeColumn(lines []pdfLine, figures []pdfFigure) []streamItem {
	linesBefore := make([]int, len(figures))
	for i, f := range figures {
		count := 0
		for _, l := range lines {
			if l.yMid() > f.top {
				count++
			}
		}
		linesBefore[i] = count
	}

	items := make([]streamItem, 0, len(lines)+len(figures))
	figIdx := 0
	for i := range lines {
		for figIdx < len(figures) && linesBefore[figIdx] == i {
			//nolint:exhaustruct // streamItem is a line/figure union; exactly one is set
			items = append(items, streamItem{figure: &figures[figIdx]})
			figIdx++
		}
		//nolint:exhaustruct // streamItem is a line/figure union; exactly one is set
		items = append(items, streamItem{line: &lines[i]})
	}
	for figIdx < len(figures) {
		//nolint:exhaustruct // streamItem is a line/figure union; exactly one is set
		items = append(items, streamItem{figure: &figures[figIdx]})
		figIdx++
	}
	return items
}

// buildPageStream assigns lines/figures to columns and merges each column's
// lines with its figures, concatenating left-column then right-column (step
// 3: reading order). A full-width figure (bounds straddle the gutter) is
// always appended after the last line of the left column.
func buildPageStream(
	lines []pdfLine,
	figures []pdfFigure,
	gutterLeft, gutterRight float64,
	twoColumn bool,
) []streamItem {
	leftLines, rightLines := assignColumns(lines, gutterLeft, gutterRight, twoColumn)

	gutterMid := (gutterLeft + gutterRight) / midpointDivisor
	var leftFigs, rightFigs, fullWidthFigs []pdfFigure
	for _, f := range figures {
		switch {
		case f.fullWidth:
			fullWidthFigs = append(fullWidthFigs, f)
		case !twoColumn || f.xMid() < gutterMid:
			leftFigs = append(leftFigs, f)
		default:
			rightFigs = append(rightFigs, f)
		}
	}

	leftStream := mergeColumn(leftLines, leftFigs)
	for i := range fullWidthFigs {
		//nolint:exhaustruct // streamItem is a line/figure union; exactly one is set
		leftStream = append(leftStream, streamItem{figure: &fullWidthFigs[i]})
	}
	rightStream := mergeColumn(rightLines, rightFigs)

	return append(leftStream, rightStream...)
}

// buildPageBlocks groups a page's stream into paragraphs/headings, applying
// hyphenation joins within each paragraph. A figure always flushes the
// current paragraph and is emitted as its own <img> block.
func buildPageBlocks(
	items []streamItem, medLineHeight, pageMedianCharWidth, docModalCharHeight float64,
) []htmlBlock {
	var blocks []htmlBlock
	var paraLines []pdfLine

	flush := func() {
		if len(paraLines) == 0 {
			return
		}
		blocks = append(blocks, renderParagraph(paraLines, docModalCharHeight))
		paraLines = nil
	}

	for _, item := range items {
		if item.figure != nil {
			flush()
			blocks = append(blocks, htmlBlock{
				html: fmt.Sprintf(
					`<img src="%s"/>`,
					escapeXMLText(item.figure.fileName),
				),
				tag:  imgTag,
				text: "",
			})
			continue
		}

		line := *item.line
		if len(paraLines) > 0 {
			prev := paraLines[len(paraLines)-1]
			if prev.col != line.col ||
				startsNewParagraph(prev, line, medLineHeight, pageMedianCharWidth) {
				flush()
			}
		}
		paraLines = append(paraLines, line)
	}
	flush()

	return blocks
}

// startsNewParagraph implements step 4: a new paragraph starts when the
// vertical gap to the previous line is too large, the current line is
// indented past the column's modal start, or the previous line ends well
// short of the column's right margin.
func startsNewParagraph(prev, cur pdfLine, medLineHeight, medCharWidth float64) bool {
	if medLineHeight > 0 && prev.bottom-cur.top > paragraphGapRatio*medLineHeight {
		return true
	}
	if medCharWidth > 0 &&
		cur.left > cur.colModalXStart+paragraphIndentChars*medCharWidth {
		return true
	}
	if prev.colRightEdge > 0 && prev.right < paragraphShortLineRatio*prev.colRightEdge {
		return true
	}
	return false
}

// renderParagraph joins a paragraph's lines (applying hyphenation, step 5)
// and classifies it as a heading (step 6) or a plain paragraph.
func renderParagraph(lines []pdfLine, docModalCharHeight float64) htmlBlock {
	text := joinLinesWithHyphenation(lines)

	heights := make([]float64, len(lines))
	for i, l := range lines {
		heights[i] = l.medianCharHeight
	}
	medHeight := median(heights)

	tag := "p"
	if docModalCharHeight > 0 {
		switch {
		case medHeight > headingH1Ratio*docModalCharHeight:
			tag = "h1"
		case medHeight > headingH2Ratio*docModalCharHeight:
			tag = "h2"
		}
	}

	return htmlBlock{
		html: fmt.Sprintf("<%s>%s</%s>", tag, escapeXMLText(text), tag),
		tag:  tag,
		text: text,
	}
}

// joinLinesWithHyphenation joins consecutive line texts with a space, except
// when a line ends with '-' and the next starts with a lowercase letter, in
// which case they're joined directly with the hyphen dropped.
func joinLinesWithHyphenation(lines []pdfLine) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0].text
	for i := 1; i < len(lines); i++ {
		cur := lines[i].text
		if strings.HasSuffix(result, "-") && startsLower(cur) {
			result = strings.TrimSuffix(result, "-") + cur
		} else {
			result = result + " " + cur
		}
	}
	return result
}

func startsLower(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLower(r)
}
