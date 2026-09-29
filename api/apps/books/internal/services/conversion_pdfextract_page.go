package services

import (
	"context"
	"fmt"
	"math"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// pageResult is one page's contribution to the document: either a full-page
// fallback image (image-only page) or a reading-order stream plus the
// typographic stats needed to group it into paragraphs later.
type pageResult struct {
	items         []streamItem
	fullPageImage string
	medLineHeight float64
	medCharWidth  float64
}

// noPageResult is the zero-value pageResult returned alongside every error
// below — a plain zero-value var (not a composite literal) so it needs no
// exhaustruct suppression.
//
//nolint:gochecknoglobals // deliberately the zero value; read-only
var noPageResult pageResult

// charHeightBucketSize is the granularity computeModalCharHeight rounds
// character heights to before finding the most common (modal) value.
const charHeightBucketSize = 0.5

// extractPage runs the full per-page pipeline: text/position extraction,
// figure extraction, the image-only-page fallback, line grouping, column
// detection, and reading-order stream assembly.
func extractPage(
	instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, index int,
	workDir string, tracker *figureTracker,
) (pageResult, error) {
	page := requests.Page{ //nolint:exhaustruct // by index
		ByIndex: &requests.PageByIndex{Document: doc, Index: index},
	}

	sizeResp, err := instance.GetPageSize(&requests.GetPageSize{Page: page})
	if err != nil {
		return noPageResult, fmt.Errorf("get page size: %w", err)
	}
	pageArea := sizeResp.Width * sizeResp.Height

	textReq := requests.GetPageTextStructured{ //nolint:exhaustruct // no pixel info
		Page: page,
		Mode: requests.GetPageTextStructuredModeChars,
		// Font name is used to detect text-run boundaries mid-line (#1653):
		// a style/font change at a word boundary doesn't reliably produce a
		// physical gap large enough for buildLine's geometric space check to
		// catch on its own.
		CollectFontInformation: true,
	}
	textResp, err := instance.GetPageTextStructured(&textReq)
	if err != nil {
		return noPageResult, fmt.Errorf("get page text: %w", err)
	}
	boxResp, err := instance.FPDF_GetPageBoundingBox(
		&requests.FPDF_GetPageBoundingBox{Page: page},
	)
	if err != nil {
		return noPageResult, fmt.Errorf("get page bounding box: %w", err)
	}
	originX, originY := float64(boxResp.Rect.Left), float64(boxResp.Rect.Bottom)

	chars := visibleChars(
		extractChars(textResp), originX, originY, sizeResp.Width, sizeResp.Height,
	)

	rawFigures, err := extractPageImages(instance, page, pageArea)
	if err != nil {
		return noPageResult, fmt.Errorf("extract page images: %w", err)
	}
	for i := range rawFigures {
		rawFigures[i].left -= originX
		rawFigures[i].right -= originX
		rawFigures[i].top -= originY
		rawFigures[i].bottom -= originY
	}

	if len(chars) < imageOnlyPageMaxChars && len(rawFigures) == 0 {
		fileName, renderErr := renderFullPage(instance, page, workDir, tracker)
		if renderErr != nil {
			return noPageResult, renderErr
		}
		// fileName is "" when the tracker rejected this raster as a
		// duplicate or over the per-document cap; the page then contributes
		// no image at all, same as a deduped regular figure.
		return pageResult{ //nolint:exhaustruct // no text stream for an image-only page
			fullPageImage: fileName,
		}, nil
	}

	gutterLeft, gutterRight, twoColumn := findGutter(
		chars,
		sizeResp.Width,
		sizeResp.Height,
	)
	lines := groupColumnLines(chars, gutterLeft, gutterRight, twoColumn)

	figures, err := placeFigures(
		rawFigures,
		gutterLeft,
		gutterRight,
		twoColumn,
		tracker,
		workDir,
	)
	if err != nil {
		return noPageResult, err
	}

	items := buildPageStream(lines, figures, gutterLeft, gutterRight, twoColumn)

	lineHeights := make([]float64, len(lines))
	for i, l := range lines {
		lineHeights[i] = l.top - l.bottom
	}

	return pageResult{
		items:         items,
		fullPageImage: "",
		medLineHeight: median(lineHeights),
		medCharWidth:  medianCharWidth(chars),
	}, nil
}

// groupColumnLines groups characters into lines, then splits every line that
// has an empty gap across the gutter into its left and right halves, so
// columns whose baselines line up (an index) never merge into one line. A
// line whose text runs across the gutter (a title above both columns) stays
// whole.
func groupColumnLines(
	chars []pdfChar, gutterLeft, gutterRight float64, twoColumn bool,
) []pdfLine {
	lines := groupLines(chars)
	if !twoColumn {
		return lines
	}
	gutterMid := (gutterLeft + gutterRight) / midpointDivisor
	var out []pdfLine
	for _, l := range lines {
		left, right, split := splitAtGutter(l.chars, gutterMid)
		if !split {
			out = append(out, l)
			continue
		}
		out = append(out, buildLine(left), buildLine(right))
	}
	return out
}

// splitAtGutter divides a line's x-sorted characters at gutterMid when no
// character box crosses it and both sides hold text.
func splitAtGutter(chars []pdfChar, gutterMid float64) ([]pdfChar, []pdfChar, bool) {
	for i, c := range chars {
		if c.left < gutterMid && c.right > gutterMid {
			return nil, nil, false
		}
		if c.left >= gutterMid {
			if i == 0 {
				return nil, nil, false
			}
			return chars[:i], chars[i:], true
		}
	}
	return nil, nil, false
}

// visibleChars moves character boxes from PDF user space into the visible
// page's space and drops characters outside it. GetPageSize reports the
// cropped page, but PDFium positions characters relative to the MediaBox
// origin: without the shift a CropBox offset (a print PDF trimmed out of a
// sheet with crop marks) skews every page-relative test, and text in the
// trimmed-off margin (proof slugs) would be read as content.
func visibleChars(chars []pdfChar, dx, dy, width, height float64) []pdfChar {
	visible := chars[:0]
	for _, c := range chars {
		c.left -= dx
		c.right -= dx
		c.top -= dy
		c.bottom -= dy
		x, y := (c.left+c.right)/midpointDivisor, c.yMid()
		if x < 0 || x > width || y < 0 || y > height {
			continue
		}
		visible = append(visible, c)
	}
	return visible
}

// extractDocument runs extractPage over every page, computes the
// document-wide modal character height needed for heading detection, and
// returns the final ordered list of HTML blocks.
func extractDocument(
	ctx context.Context,
	instance pdfium.Pdfium,
	doc references.FPDF_DOCUMENT,
	workDir string,
) ([]htmlBlock, error) {
	countResp, err := instance.FPDF_GetPageCount(
		&requests.FPDF_GetPageCount{Document: doc},
	)
	if err != nil {
		return nil, fmt.Errorf("get page count: %w", err)
	}

	tracker := newFigureTracker()
	pages := make([]pageResult, 0, countResp.PageCount)
	for i := range countResp.PageCount {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		pr, pageErr := extractPage(instance, doc, i, workDir, tracker)
		if pageErr != nil {
			return nil, fmt.Errorf("extract page %d: %w", i, pageErr)
		}
		pages = append(pages, pr)
	}

	docModalHeight := computeModalCharHeight(pages)
	rebuildHeadingLineText(pages, docModalHeight)

	pageBlocks := make([][]htmlBlock, len(pages))
	for i, p := range pages {
		if p.fullPageImage != "" {
			alt := fmt.Sprintf("Page %d illustration", i+1)
			pageBlocks[i] = []htmlBlock{
				{ //nolint:exhaustruct // medHeight/isText only apply to text blocks
					html: fmt.Sprintf(
						`<img src="%s" alt="%s"/>`,
						escapeXMLText(p.fullPageImage),
						escapeXMLText(alt),
					),
					tag:  imgTag,
					text: "",
				},
			}
			continue
		}
		pageBlocks[i] = buildPageBlocks(
			p.items,
			p.medLineHeight,
			p.medCharWidth,
		)
	}
	pageBlocks = removeProofSlugLines(pageBlocks)

	var blocks []htmlBlock
	for _, pb := range pageBlocks {
		blocks = append(blocks, pb...)
	}
	finalizeHeadings(blocks, docModalHeight)
	return blocks, nil
}

// computeModalCharHeight finds the document's most common line character
// height (bucketed to the nearest charHeightBucketSize), used as the "body
// text" baseline for heading detection.
func computeModalCharHeight(pages []pageResult) float64 {
	freq := map[float64]int{}
	for _, p := range pages {
		for _, item := range p.items {
			if item.line == nil {
				continue
			}
			bucket := math.Round(
				item.line.medianCharHeight/charHeightBucketSize,
			) * charHeightBucketSize
			freq[bucket]++
		}
	}

	best, bestCount := 0.0, 0
	for v, count := range freq {
		if count > bestCount || (count == bestCount && v < best) {
			best, bestCount = v, count
		}
	}
	return best
}
