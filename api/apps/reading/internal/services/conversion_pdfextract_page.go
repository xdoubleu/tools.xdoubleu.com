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

	textReq := requests.GetPageTextStructured{ //nolint:exhaustruct // no font/pixel info
		Page: page,
		Mode: requests.GetPageTextStructuredModeChars,
	}
	textResp, err := instance.GetPageTextStructured(&textReq)
	if err != nil {
		return noPageResult, fmt.Errorf("get page text: %w", err)
	}
	chars := extractChars(textResp)

	rawFigures, err := extractPageImages(instance, page, pageArea)
	if err != nil {
		return noPageResult, fmt.Errorf("extract page images: %w", err)
	}

	if len(chars) < imageOnlyPageMaxChars && len(rawFigures) == 0 {
		fileName, renderErr := renderFullPage(instance, page, index, workDir)
		if renderErr != nil {
			return noPageResult, renderErr
		}
		return pageResult{ //nolint:exhaustruct // no text stream for an image-only page
			fullPageImage: fileName,
		}, nil
	}

	lines := groupLines(chars)
	gutterLeft, gutterRight, twoColumn := findGutter(
		chars,
		sizeResp.Width,
		sizeResp.Height,
	)

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

	var blocks []htmlBlock
	for _, p := range pages {
		if p.fullPageImage != "" {
			blocks = append(blocks, htmlBlock{
				html: fmt.Sprintf(`<img src="%s"/>`, escapeXMLText(p.fullPageImage)),
				tag:  imgTag,
				text: "",
			})
			continue
		}
		blocks = append(
			blocks,
			buildPageBlocks(
				p.items,
				p.medLineHeight,
				p.medCharWidth,
				docModalHeight,
			)...,
		)
	}
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
