package services

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
)

// Vector figures (diagrams and graphs drawn with path objects) have no image
// object to extract: their region of the page is rendered instead, and the
// labels drawn over and around them are taken out of the text.
const (
	// vectorClusterGap joins path boxes this close (in points) into one
	// figure.
	vectorClusterGap = 8.0
	// vectorMinSide drops clusters thinner than this: rules and underlines.
	vectorMinSide = 20.0
	// vectorMaxPageShare drops paths covering most of the page: a page
	// background, not a figure.
	vectorMaxPageShare = 0.9
	// vectorProseWidthShare/vectorProseMinWords: a cluster enclosing a
	// non-label line at least this share of its width and this many words
	// long is a box around text (a shaded sidebar), not a figure.
	vectorProseWidthShare = 0.5
	vectorProseMinWords   = 5
	// vectorLabelMaxWords/vectorLabelReach: a line of at most this many
	// words, or of tick values only, within this many line heights of a
	// figure (vectorSideLabelReach beside it) is one of its labels, taken
	// into the figure.
	vectorLabelMaxWords  = 4
	vectorLabelReach     = 2.5
	vectorSideLabelReach = 5.0
	// vectorLabelGapChars splits a line into separate labels at gaps this
	// many median character widths wide.
	vectorLabelGapChars = 3.0
	// vectorRegionMergeGap joins figure regions this close (in points)
	// with no text between them: panels of one figure.
	vectorRegionMergeGap = 20.0
	// vectorPadding pads a figure's rendered region, in points.
	vectorPadding = 3.0
	// pointsPerInch converts render DPI to pixels per point.
	pointsPerInch = 72.0
)

// box is a rectangle in page space (origin bottom-left).
type box struct {
	left, bottom, right, top float64
}

func (b box) width() float64 { return b.right - b.left }

func (b box) height() float64 { return b.top - b.bottom }

func (b box) union(o box) box {
	return box{
		left: min(b.left, o.left), bottom: min(b.bottom, o.bottom),
		right: max(b.right, o.right), top: max(b.top, o.top),
	}
}

// distance is the gap between two boxes, 0 when they overlap.
func (b box) distance(o box) float64 {
	dx := max(0, o.left-b.right, b.left-o.right)
	dy := max(0, o.bottom-b.top, b.bottom-o.top)
	return max(dx, dy)
}

func (b box) contains(x, y float64) bool {
	return x >= b.left && x <= b.right && y >= b.bottom && y <= b.top
}

func lineBox(l pdfLine) box {
	return box{left: l.left, bottom: l.bottom, right: l.right, top: l.top}
}

// pageGeometry is the visible page's size and its offset in PDF user space.
type pageGeometry struct {
	originX, originY, width, height float64
}

// extractVectorFigures renders each vector figure on the page and returns
// it with the characters left once its labels are removed.
func extractVectorFigures(
	instance pdfium.Pdfium, page requests.Page, geo pageGeometry, chars []pdfChar,
) ([]rawFigure, []pdfChar, error) {
	boxes, err := pathBoxes(instance, page, geo)
	if err != nil || len(boxes) == 0 {
		return nil, chars, err
	}

	lines := groupLines(append([]pdfChar(nil), chars...))
	taken := make([]bool, len(lines))
	var regions []box
	for _, r := range clusterBoxes(boxes) {
		if r.width() < vectorMinSide || r.height() < vectorMinSide ||
			r.width()*r.height() < figureMinAreaFraction*geo.width*geo.height {
			continue
		}
		if region, ok := claimFigureRegion(r, lines, taken); ok {
			regions = append(regions, region)
		}
	}
	if len(regions) == 0 {
		return nil, chars, nil
	}
	regions = mergePanels(regions, lines, taken)

	figures, err := renderRegions(instance, page, geo, regions)
	if err != nil {
		return nil, chars, err
	}
	return figures, charsOutsideLines(chars, lines, taken), nil
}

// pathBoxes returns the visible bounds of the page's path objects, in page
// space. Paths off the page (crop marks) and page-sized backgrounds don't
// count.
func pathBoxes(
	instance pdfium.Pdfium, page requests.Page, geo pageGeometry,
) ([]box, error) {
	countResp, err := instance.FPDFPage_CountObjects(
		&requests.FPDFPage_CountObjects{Page: page},
	)
	if err != nil {
		return nil, fmt.Errorf("count page objects: %w", err)
	}
	var boxes []box
	for i := range countResp.Count {
		objResp, getErr := instance.FPDFPage_GetObject(
			&requests.FPDFPage_GetObject{Page: page, Index: i},
		)
		if getErr != nil {
			return nil, fmt.Errorf("get page object %d: %w", i, getErr)
		}
		typeResp, typeErr := instance.FPDFPageObj_GetType(
			&requests.FPDFPageObj_GetType{PageObject: objResp.PageObject},
		)
		if typeErr != nil {
			return nil, fmt.Errorf("get page object type %d: %w", i, typeErr)
		}
		if typeResp.Type != enums.FPDF_PAGEOBJ_PATH {
			continue
		}
		b, boundsErr := instance.FPDFPageObj_GetBounds(
			&requests.FPDFPageObj_GetBounds{PageObject: objResp.PageObject},
		)
		if boundsErr != nil {
			return nil, fmt.Errorf("get path bounds %d: %w", i, boundsErr)
		}
		visible := box{
			left:   max(0, float64(b.Left)-geo.originX),
			bottom: max(0, float64(b.Bottom)-geo.originY),
			right:  min(geo.width, float64(b.Right)-geo.originX),
			top:    min(geo.height, float64(b.Top)-geo.originY),
		}
		if visible.width() < 0 || visible.height() < 0 ||
			visible.width()*visible.height() > vectorMaxPageShare*geo.width*geo.height {
			continue
		}
		boxes = append(boxes, visible)
	}
	return boxes, nil
}

// clusterBoxes merges boxes within vectorClusterGap of each other until no
// two clusters are that close.
func clusterBoxes(boxes []box) []box {
	clusters := append([]box(nil), boxes...)
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(clusters); i++ {
			for j := i + 1; j < len(clusters); j++ {
				if clusters[i].distance(clusters[j]) <= vectorClusterGap {
					clusters[i] = clusters[i].union(clusters[j])
					clusters = append(clusters[:j], clusters[j+1:]...)
					merged = true
					j--
				}
			}
		}
	}
	return clusters
}

// renderRegions renders the page once and crops each region out of it.
func renderRegions(
	instance pdfium.Pdfium, page requests.Page, geo pageGeometry, regions []box,
) ([]rawFigure, error) {
	renderResp, err := instance.RenderPageInDPI(
		&requests.RenderPageInDPI{ //nolint:exhaustruct // defaults suit a plain page render
			Page: page,
			DPI:  fullPageRenderDPI,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("render page for figures: %w", err)
	}
	defer renderResp.Cleanup()

	img, ok := renderResp.Result.RenderedImage.(interface {
		image.Image
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("render page for figures: unexpected image type %T",
			renderResp.Result.RenderedImage)
	}
	scale := fullPageRenderDPI / pointsPerInch

	figures := make([]rawFigure, 0, len(regions))
	for _, r := range regions {
		r = box{
			left: max(0, r.left-vectorPadding), bottom: max(0, r.bottom-vectorPadding),
			right: min(geo.width, r.right+vectorPadding),
			top:   min(geo.height, r.top+vectorPadding),
		}
		px := image.Rect(
			int(r.left*scale), int((geo.height-r.top)*scale),
			int(r.right*scale), int((geo.height-r.bottom)*scale),
		).Intersect(img.Bounds())
		var buf bytes.Buffer
		if err = png.Encode(&buf, img.SubImage(px)); err != nil {
			return nil, fmt.Errorf("encode figure png: %w", err)
		}
		figures = append(figures, rawFigure{
			png: buf.Bytes(), left: r.left, top: r.top, right: r.right, bottom: r.bottom,
		})
	}
	return figures, nil
}
