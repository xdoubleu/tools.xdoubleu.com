package services

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Step 6 (headings): a paragraph taller than these ratios of the document's
// modal body height becomes <h1>/<h2>.
const (
	headingH1Ratio = 1.4
	headingH2Ratio = 1.15
	// Step 4 (paragraphs).
	paragraphGapRatio    = 1.5
	paragraphIndentChars = 2
	// A line ending short of the right margin by more than this ends its paragraph.
	paragraphShortLineRatio = 0.85
)

// streamItem is a text line or a figure in a page's reading order.
type streamItem struct {
	line   *pdfLine
	figure *pdfFigure
}

// htmlBlock is one paragraph, heading, or image block.
type htmlBlock struct {
	html string
	tag  string // "p", "h1", "h2", or "img" — used for title fallback/tests.
	text string // raw (unescaped) text; empty for "img".
	// medHeight and isText are set only for paragraph blocks, for finalizeHeadings
	// to classify with document-wide context.
	medHeight float64
	isText    bool
}

// mergeColumn inserts each figure after the last line whose y-midpoint is above
// the figure's top.
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

// buildPageStream merges each column's lines and figures, left column first.
// A full-width figure goes after the left column.
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

// buildPageBlocks groups a page's stream into paragraphs; a figure flushes the
// current paragraph. Headings are classified later by finalizeHeadings.
func buildPageBlocks(
	items []streamItem, medLineHeight, pageMedianCharWidth float64,
) []htmlBlock {
	var blocks []htmlBlock
	var paraLines []pdfLine
	figureCount := 0

	flush := func() {
		if len(paraLines) == 0 {
			return
		}
		blocks = append(blocks, renderParagraph(paraLines))
		paraLines = nil
	}

	for _, item := range items {
		if item.figure != nil {
			flush()
			figureCount++
			blocks = append(
				blocks,
				htmlBlock{ //nolint:exhaustruct // medHeight/isText only apply to text blocks
					html: fmt.Sprintf(
						`<img src="%s" alt="%s"/>`,
						escapeXMLText(item.figure.fileName),
						escapeXMLText(fmt.Sprintf("Figure %d", figureCount)),
					),
					tag:  imgTag,
					text: "",
				},
			)
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

// startsNewParagraph: a large vertical gap, an indent past the column's modal
// start, or a short previous line.
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

// renderParagraph joins a paragraph's lines (dehyphenating) into a "p" block.
func renderParagraph(lines []pdfLine) htmlBlock {
	text := joinLinesWithHyphenation(lines)

	heights := make([]float64, len(lines))
	for i, l := range lines {
		heights[i] = l.medianCharHeight
	}
	medHeight := median(heights)

	return htmlBlock{
		html:      "", // filled in by finalizeHeadings
		tag:       "p",
		text:      text,
		medHeight: medHeight,
		isText:    true,
	}
}

// finalizeHeadings assigns each paragraph's final tag by height ratio, then
// demotes candidates that aren't real headings: list-like runs
// (demoteHeadingRuns) first, then survivors that start lowercase, are
// loop-diagram labels ("R B"), or fail isHeadingFalsePositive.
func finalizeHeadings(blocks []htmlBlock, docModalCharHeight float64) {
	tags := make([]string, len(blocks))
	for i, b := range blocks {
		if !b.isText {
			continue
		}
		tags[i] = headingCandidateTag(b.medHeight, docModalCharHeight)
	}

	demoteHeadingRuns(blocks, tags)

	for i, b := range blocks {
		if !b.isText || tags[i] == "p" {
			continue
		}
		if startsLowercase(b.text) || isLoopDiagramLabel(b.text) {
			tags[i] = "p"
			continue
		}
		// Sentence punctuation and figure references demote both sizes. The one-word
		// mid-band guard is h1-only: a one-word h2 is a normal section heading, while
		// a mid-size one-word h1 is a figure label ("Cooling").
		if isHeadingFalsePositive(
			b.text, b.medHeight, docModalCharHeight, tags[i] == "h1",
		) {
			tags[i] = "p"
		}
	}

	for i := range blocks {
		if !blocks[i].isText {
			continue
		}
		blocks[i].tag = tags[i]
		blocks[i].html = fmt.Sprintf(
			"<%s>%s</%s>", tags[i], escapeXMLText(blocks[i].text), tags[i],
		)
	}
}

func headingCandidateTag(medHeight, docModalCharHeight float64) string {
	if docModalCharHeight <= 0 {
		return "p"
	}
	switch {
	case medHeight > headingH1Ratio*docModalCharHeight:
		return "h1"
	case medHeight > headingH2Ratio*docModalCharHeight:
		return "h2"
	default:
		return "p"
	}
}

func startsLowercase(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return r != utf8.RuneError && unicode.IsLower(r)
}

// loopDiagramLabelRe matches single uppercase letters ("B", "R B"): a
// systems-diagram loop label.
var loopDiagramLabelRe = regexp.MustCompile(`^[A-Z](?: [A-Z])*$`)

func isLoopDiagramLabel(text string) bool {
	return loopDiagramLabelRe.MatchString(strings.TrimSpace(text))
}

// headingH1MidBandRatio: real one-word headings render at title size, well
// above this; diagram labels sit just above the h1 threshold.
const headingH1MidBandRatio = 1.6

// closingQuoteChars may follow final punctuation without changing its shape.
const closingQuoteChars = "”’\"')]}»·"

// trailingEllipsisRe matches a trailing ellipsis, which a title may end in.
var trailingEllipsisRe = regexp.MustCompile(`(?:\s*\.\s*){3,}$`)

// endsSentencePunctuation reports whether text ends in sentence punctuation,
// ignoring a trailing ellipsis and closing quotes.
func endsSentencePunctuation(text string) bool {
	stripped := strings.TrimRight(strings.TrimSpace(text), closingQuoteChars)
	stripped = strings.TrimRight(stripped, "…")
	stripped = trailingEllipsisRe.ReplaceAllString(stripped, "")
	if stripped == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(stripped)
	switch r {
	case '.', '?', ',', ';', '!':
		return true
	default:
		return false
	}
}

// figureRefRe marks a caption or cross-reference rather than a heading.
var figureRefRe = regexp.MustCompile(`Figure \d`)

// isHeadingFalsePositive reports whether a candidate's text shape rules it
// out; isH1 enables the one-word mid-band guard.
func isHeadingFalsePositive(
	text string, medHeight, docModalCharHeight float64, isH1 bool,
) bool {
	trimmed := strings.TrimSpace(text)
	if endsSentencePunctuation(trimmed) || figureRefRe.MatchString(trimmed) {
		return true
	}
	if isH1 && docModalCharHeight > 0 &&
		medHeight < headingH1MidBandRatio*docModalCharHeight &&
		len(strings.Fields(trimmed)) == 1 {
		return true
	}
	return false
}

// demoteHeadingRuns flattens chains of consecutive similar-size heading
// candidates: 3+ is a bibliography/list page; a pair only when one member is
// list-shaped (listShapedPair), so a title page's banner+title or a two-line
// title survives. A non-paragraph block breaks a chain.
func demoteHeadingRuns(blocks []htmlBlock, tags []string) {
	chainStart := -1
	prevHeight := 0.0
	flush := func(end int) {
		if chainStart >= 0 {
			n := end - chainStart
			if n >= 3 || (n == 2 && listShapedPair(blocks, chainStart)) {
				for i := chainStart; i < end; i++ {
					tags[i] = "p"
				}
			}
		}
		chainStart = -1
	}
	for i, b := range blocks {
		if !b.isText || tags[i] == "p" {
			flush(i)
			continue
		}
		if chainStart < 0 {
			chainStart = i
			prevHeight = b.medHeight
			continue
		}
		if heightsSimilar(prevHeight, b.medHeight) {
			prevHeight = b.medHeight
			continue
		}
		flush(i)
		chainStart = i
		prevHeight = b.medHeight
	}
	if chainStart >= 0 {
		flush(len(blocks))
	}
}

// listShapedPair reports whether either member of a two-member chain has a
// list entry's text shape.
func listShapedPair(blocks []htmlBlock, start int) bool {
	for i := start; i < start+2; i++ {
		if endsSentencePunctuation(blocks[i].text) ||
			figureRefRe.MatchString(blocks[i].text) ||
			startsLowercase(blocks[i].text) ||
			isLoopDiagramLabel(blocks[i].text) {
			return true
		}
	}
	return false
}

// headingRunSimilarity is the max relative height difference for two
// candidates to count as the same font size.
const headingRunSimilarity = 0.1

func heightsSimilar(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	if a < b {
		a, b = b, a
	}
	return a-b <= headingRunSimilarity*b
}

// joinLinesWithHyphenation joins lines with spaces, except a trailing '-'
// before a lowercase start joins directly with the hyphen dropped.
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
