package services

import (
	"fmt"
	"regexp"
	"sort"
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
)

// asideTag is the htmlBlock tag of a margin note.
const asideTag = "blockquote"

// streamItem is one entry in a page's reading-order stream: a line of text,
// a figure placed between lines, or a margin note (aside) beside them.
type streamItem struct {
	line   *pdfLine
	figure *pdfFigure
	aside  []pdfLine
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
	// listItem marks a bulleted paragraph, rendered as <li> and never a
	// heading.
	listItem bool
	// src is an image block's file name.
	src string
	// inline is a paragraph's text as escaped HTML with inline markup.
	inline string
	// tocLevel is the heading level a PDF outline entry gives the block
	// (0: not in the outline).
	tocLevel int
}

// imageBlock renders an <img> block.
func imageBlock(src, alt string) htmlBlock {
	return htmlBlock{ //nolint:exhaustruct // medHeight/isText only apply to text blocks
		html: fmt.Sprintf(
			`<img src="%s" alt="%s"/>`,
			escapeXMLText(src),
			escapeAttr(alt),
		),
		tag: imgTag,
		src: src,
	}
}

// escapeAttr escapes text for a double-quoted XML attribute value.
func escapeAttr(s string) string {
	return strings.ReplaceAll(escapeXMLText(s), `"`, "&quot;")
}

// captionFigures gives each image the caption right after it ("Figure 10.
// A cup of coffee cooling…") as its alt text.
func captionFigures(blocks []htmlBlock) {
	for i := range blocks {
		if blocks[i].tag != imgTag || i+1 >= len(blocks) {
			continue
		}
		if next := blocks[i+1]; next.isText && captionRe.MatchString(next.text) {
			blocks[i] = imageBlock(blocks[i].src, next.text)
		}
	}
}

// captionRe matches a figure caption's opening.
var captionRe = regexp.MustCompile(`^(?:Figure|Fig\.|Table) \d+`)

// mergeColumn interleaves a column's lines (already sorted top-to-bottom)
// with its figures, inserting each figure after the last line whose
// y-midpoint is above the figure's top bound (step: figure placement).
func mergeColumn(lines []pdfLine, figures []pdfFigure) []streamItem {
	figures = append([]pdfFigure(nil), figures...)
	sort.SliceStable(
		figures,
		func(i, j int) bool { return figures[i].top > figures[j].top },
	)
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
			//nolint:exhaustruct // streamItem is a line/figure/aside union
			items = append(items, streamItem{figure: &figures[figIdx]})
			figIdx++
		}
		//nolint:exhaustruct // streamItem is a line/figure/aside union
		items = append(items, streamItem{line: &lines[i]})
	}
	for figIdx < len(figures) {
		//nolint:exhaustruct // streamItem is a line/figure/aside union
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
	leftLines, leftNotes := separateAsides(leftLines)
	rightLines, rightNotes := separateAsides(rightLines)
	applyColStats(leftLines, 0)
	applyColStats(rightLines, 1)

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

	leftStream := insertAsides(mergeColumn(leftLines, leftFigs), leftNotes)
	for i := range fullWidthFigs {
		//nolint:exhaustruct // streamItem is a line/figure/aside union
		leftStream = append(leftStream, streamItem{figure: &fullWidthFigs[i]})
	}
	rightStream := insertAsides(mergeColumn(rightLines, rightFigs), rightNotes)

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
	var pendingNotes [][]pdfLine
	setLocalRightEdges(items, pageMedianCharWidth)
	hanging := hangingColumns(items, pageMedianCharWidth)
	index := indexColumns(items)

	flush := func() {
		if len(paraLines) > 0 {
			blocks = append(blocks, asListItem(renderParagraph(paraLines)))
			paraLines = nil
		}
		for _, note := range pendingNotes {
			blocks = append(blocks, renderAside(note, paraRules{
				medLineHeight: medLineHeight,
				medCharWidth:  pageMedianCharWidth,
				hanging:       false,
				index:         false,
			}))
		}
		pendingNotes = nil
	}

	for _, item := range items {
		if item.aside != nil {
			// A note sits beside a paragraph without ending it; it follows
			// the paragraph once that ends.
			pendingNotes = append(pendingNotes, item.aside)
			continue
		}
		if item.figure != nil {
			flush()
			blocks = append(blocks, imageBlock(item.figure.fileName, "Figure"))
			continue
		}

		line := *item.line
		if len(paraLines) > 0 {
			rules := paraRules{
				medLineHeight: medLineHeight,
				medCharWidth:  pageMedianCharWidth,
				hanging:       hanging[line.col],
				index:         index[line.col],
			}
			if paraLines[len(paraLines)-1].col != line.col ||
				startsNewParagraph(paraLines, line, rules) {
				flush()
			}
		}
		paraLines = append(paraLines, line)
	}
	flush()

	return blocks
}

// renderAside renders a margin note as a blockquote of its own paragraphs.
func renderAside(lines []pdfLine, rules paraRules) htmlBlock {
	var b strings.Builder
	var texts []string
	para := []pdfLine{lines[0]}
	emit := func() {
		texts = append(texts, joinLinesWithHyphenation(para))
		b.WriteString("<p>" + paragraphHTML(para) + "</p>")
	}
	for _, l := range lines[1:] {
		if startsNewParagraph(para, l, rules) {
			emit()
			para = nil
		}
		para = append(para, l)
	}
	emit()
	return htmlBlock{ //nolint:exhaustruct // medHeight/isText only apply to text blocks
		html: "<blockquote>" + b.String() + "</blockquote>",
		tag:  asideTag,
		text: strings.Join(texts, " "),
	}
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
		listItem:  false,
		src:       "",
		inline:    paragraphHTML(lines),
		tocLevel:  0,
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
