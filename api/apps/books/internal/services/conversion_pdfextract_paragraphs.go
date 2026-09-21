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
// ratio to the document's modal (body text) character height, as before, but
// a candidate heading is then demoted to "p" when it's part of a run of 2+
// consecutive large-font paragraphs: a real heading tends to be isolated,
// surrounded by ordinary body-text paragraphs, whereas a bibliography- or
// list-style block (e.g. a frontmatter "Other books by this author" page)
// shares its larger font across many consecutive entries. Without this guard,
// such lists were misclassified as headings roughly two orders of magnitude
// more often than real chapter/section boundaries occur (issue #1654). A
// non-paragraph block (e.g. an image) breaks a run, since it can't itself be
// a citation-list entry. A candidate whose text starts with a lowercase
// letter is demoted to "p" outright, since a real title/heading never starts
// mid-sentence — this catches large-font marginal pull-quote words and
// run-on paragraph fragments that the height ratio alone misclassifies
// (issue #1698). A candidate consisting only of single uppercase-letter
// tokens (e.g. "B", "R B", "B B") is demoted the same way: a real
// title/heading is never shaped like that, but a systems-diagram's loop
// labels (B for a balancing loop, R for reinforcing) are, and large embedded
// diagram callouts push their height ratio well past the heading threshold
// (issue #1698 follow-up left open by #1766).
func finalizeHeadings(blocks []htmlBlock, docModalCharHeight float64) {
	tags := make([]string, len(blocks))
	for i, b := range blocks {
		if !b.isText {
			continue
		}
		tags[i] = headingCandidateTag(b.medHeight, docModalCharHeight)
		if tags[i] != "p" && (startsLowercase(b.text) || isLoopDiagramLabel(b.text)) {
			tags[i] = "p"
		}
	}

	demoteHeadingRuns(blocks, tags)

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

// demoteHeadingRuns rewrites tags in place, flattening any run of 2+
// consecutive non-"p" text blocks down to "p" — see finalizeHeadings.
func demoteHeadingRuns(blocks []htmlBlock, tags []string) {
	runStart := -1
	flush := func(end int) {
		if runStart >= 0 && end-runStart > 1 {
			for i := runStart; i < end; i++ {
				tags[i] = "p"
			}
		}
		runStart = -1
	}
	for i, b := range blocks {
		if !b.isText || tags[i] == "p" {
			flush(i)
			continue
		}
		if runStart < 0 {
			runStart = i
		}
	}
	flush(len(blocks))
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
