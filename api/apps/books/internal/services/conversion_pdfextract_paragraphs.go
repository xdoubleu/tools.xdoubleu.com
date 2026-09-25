package services

import (
	"fmt"
	"regexp"
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
	// medHeight and isText are populated only for paragraph blocks
	// (renderParagraph), never "img" blocks, and are consumed by
	// finalizeHeadings — which assigns the final tag/html using
	// document-wide context to avoid misclassifying bibliography/list-style
	// large text as a heading (issue #1654).
	medHeight float64
	isText    bool
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

// buildPageBlocks groups a page's stream into paragraphs, applying
// hyphenation joins within each paragraph (heading classification is
// deferred to finalizeHeadings, which needs document-wide context — see
// conversion_pdfextract_page.go's extractDocument). A figure always flushes
// the current paragraph and is emitted as its own <img> block.
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
// into a plain-paragraph block, carrying its median character height for
// finalizeHeadings to classify (step 6) once every paragraph in the document
// is known.
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

// finalizeHeadings assigns each paragraph block's final tag ("h1"/"h2"/"p")
// and renders its html, in place. A paragraph is classified purely by height
// ratio to the document's modal (body text) character height, as before.
// A candidate that passes the ratio test is then demoted to "p" when its
// shape or context says it isn't a real heading:
//
//   - startsLowercase: a real title never starts mid-sentence — catches
//     large-font marginal pull-quote words and run-on paragraph fragments
//     (issue #1698).
//   - isLoopDiagramLabel: text made up only of single uppercase-letter
//     tokens ("B", "R B", "B B") is a systems-diagram loop-label callout,
//     never a title (issue #1698 follow-up left open by #1766).
//   - startsLowercase: a real title never starts mid-sentence — catches
//     large-font marginal pull-quote words and run-on paragraph fragments
//     (issue #1698).
//   - isLoopDiagramLabel: text made up only of single uppercase-letter
//     tokens ("B", "R B", "B B") is a systems-diagram loop-label callout,
//     never a title (issue #1698 follow-up left open by #1766).
//   - isHeadingFalsePositive: a candidate ending in sentence punctuation
//     (a quoted pull quote, a numbered list item, a question fragment) or
//     referencing a figure number ("Delays, Figure 30:", "(see Figure 39)")
//     is body text or a caption; an h1 candidate that is a single word at
//     mid-band size (above body text but well below real title-page type)
//     is an embedded diagram label ("Cooling") (issue #1698).
//
// The shape guards run after demoteHeadingRuns so that a run member already
// flattened as a list entry can't "shield" its neighbours: every member of a
// run goes to "p" regardless of its own shape, and the guards judge only
// survivors in isolation.
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
		// Sentence punctuation and figure references are decisive across
		// both heading sizes — no real title/section heading ends in a
		// sentence or cites a figure number. The one-word mid-band guard
		// stays h1-only: a one-word h2 (above body text but under the h1
		// threshold) is a normal small section heading ("Resilience"),
		// while a one-worder that clears the h1 threshold at mid-band size
		// is an embedded figure label ("Cooling").
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

// headingCandidateTag classifies a single paragraph by height ratio alone,
// ignoring surrounding context (that's demoteHeadingRuns's job).
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

// startsLowercase reports whether text's first rune is a lowercase letter.
// Text with no leading letter (empty, or starting with punctuation/digits)
// is not considered lowercase-started.
func startsLowercase(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return r != utf8.RuneError && unicode.IsLower(r)
}

// loopDiagramLabelRe matches text made up of one or more single uppercase
// ASCII letters separated by single spaces ("B", "R B", "B B", …) — the
// shape of a systems-diagram loop-label callout, never of a real
// title/heading (see finalizeHeadings, issue #1698).
var loopDiagramLabelRe = regexp.MustCompile(`^[A-Z](?: [A-Z])*$`)

// isLoopDiagramLabel reports whether text (after trimming surrounding
// whitespace) matches loopDiagramLabelRe.
func isLoopDiagramLabel(text string) bool {
	return loopDiagramLabelRe.MatchString(strings.TrimSpace(text))
}

// headingH1MidBandRatio bounds the size band in which a one-word h1
// candidate is treated as an embedded figure label: real one-word headings
// ("Appendix", chapter titles) are rendered at title-page size, well above
// this ratio, while diagram labels ("Cooling") sit just above the h1
// threshold.
const headingH1MidBandRatio = 1.6

// closingQuoteChars are closing punctuation that may follow a sentence's
// final punctuation mark without changing its shape ("…like a pillar.”").
const closingQuoteChars = "”’\"')]}»·"

// trailingEllipsisRe matches a trailing typographic ellipsis, spaced dots
// ("System Traps . . .") or "…" — a title can end in one, so it doesn't
// make the text sentence-shaped.
var trailingEllipsisRe = regexp.MustCompile(`(?:\s*\.\s*){3,}$`)

// endsSentencePunctuation reports whether trimmed heading-candidate text
// ends in sentence punctuation once a trailing ellipsis and trailing
// closing quotes/brackets are stripped — the shape of a sentence, numbered
// list item, or question fragment, never of a real title/heading.
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

// figureRefRe matches a figure-number reference ("Figure 39", "Figure 1 2"),
// the marker of a caption or cross-reference fragment rather than a heading.
var figureRefRe = regexp.MustCompile(`Figure \d`)

// isHeadingFalsePositive reports whether a heading candidate's text shape
// says it isn't a heading (see finalizeHeadings). isH1 controls the
// one-word mid-band guard, which applies to h1 candidates only.
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

// demoteHeadingRuns rewrites tags in place, flattening chains of
// consecutive non-"p" text blocks that look like a list (see
// finalizeHeadings): candidates chain while consecutive AND rendered at a
// similar size, and a chain of 3+ is a bibliography/citation page
// regardless of its members' text shapes (issue #1654), while a 2-member
// chain counts as a list only when one member has a list entry's text
// shape (see listShapedPair) — a chapter title page's decoration banner
// (h2-sized) beside its h1-sized title is a dissimilar pair, and a
// two-line title at one size has no entry shape; flattening either would
// erase the TOC's chapter entry (issue #1698). A non-paragraph block (e.g.
// an image) breaks a chain.
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
		// A size jump splits the chain: decide the chain so far, start a
		// new one here.
		flush(i)
		chainStart = i
		prevHeight = b.medHeight
	}
	if chainStart >= 0 {
		flush(len(blocks))
	}
}

// listShapedPair reports whether a two-member same-size chain counts as
// list entries: at least one member has an entry's text shape (sentence
// punctuation, a figure reference, a lowercase start, or a loop-label
// shape). Without the shape requirement, a two-line title rendered at one
// size ("Leverage Points—" / "Places to I ntervene in a System") would be
// flattened and vanish from the chapter TOC.
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

// headingRunSimilarity is the maximum relative height difference two
// adjacent heading candidates may have and still count as list entries
// rendered in the same font size.
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
