package services

import (
	"context"
	"fmt"
	"math"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// pageResult is a full-page fallback image or a reading-order stream with
// its typographic stats.
type pageResult struct {
	items         []streamItem
	fullPageImage string
	medLineHeight float64
	medCharWidth  float64
}

//nolint:gochecknoglobals // deliberately the zero value; read-only
var noPageResult pageResult

const charHeightBucketSize = 0.5

// extractPage runs the per-page pipeline through reading-order assembly.
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
		// Font names expose run boundaries that leave no measurable gap.
		CollectFontInformation: true,
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
		fileName, renderErr := renderFullPage(instance, page, workDir, tracker)
		if renderErr != nil {
			return noPageResult, renderErr
		}
		// "" means the tracker rejected the raster (duplicate or over cap).
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

// extractDocument extracts every page and returns the final ordered blocks.
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

// computeModalCharHeight returns the most common line char height (the body
// baseline for heading detection).
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
