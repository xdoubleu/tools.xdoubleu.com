package services

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// figureMinPixels/figureMinAreaFraction/figureMaxPerDoc implement the figure
// filtering rules: drop rules/bullets/glyph fragments by pixel size, drop
// anything covering too little of the page, and cap the total kept per
// document.
const (
	figureMinPixels       = 50
	figureMinAreaFraction = 0.01
	figureMaxPerDoc       = 50
	fullPageRenderDPI     = 150
	imageOnlyPageMaxChars = 200
)

// rawFigure is a candidate figure image that has passed the per-object
// filters but not yet the document-level dedupe/cap.
type rawFigure struct {
	png                      []byte
	left, top, right, bottom float64
}

// pdfFigure is a figure placed in the page's reading-order stream, already
// written to workDir under fileName.
type pdfFigure struct {
	fileName                 string
	left, top, right, bottom float64
	fullWidth                bool
}

func (f pdfFigure) xMid() float64 { return (f.left + f.right) / 2 }

// figureTracker dedupes figures by the SHA-256 of their encoded PNG across
// the whole document and enforces the per-document cap.
type figureTracker struct {
	seen  map[[32]byte]bool
	count int
}

func newFigureTracker() *figureTracker {
	return &figureTracker{seen: map[[32]byte]bool{}}
}

// accept returns the filename to use for pngData and true if it should be
// kept (first occurrence, under the cap), or "", false otherwise.
func (t *figureTracker) accept(pngData []byte) (string, bool) {
	if t.count >= figureMaxPerDoc {
		return "", false
	}
	hash := sha256.Sum256(pngData)
	if t.seen[hash] {
		return "", false
	}
	t.seen[hash] = true
	name := fmt.Sprintf("fig-%d.png", t.count)
	t.count++
	return name, true
}

// extractPageImages enumerates a page's objects, keeping image objects that
// survive the pixel-size and page-area filters (steps performed before
// dedupe, which is a document-level concern).
func extractPageImages(
	instance pdfium.Pdfium, page requests.Page, pageArea float64,
) ([]rawFigure, error) {
	countResp, err := instance.FPDFPage_CountObjects(
		&requests.FPDFPage_CountObjects{Page: page},
	)
	if err != nil {
		return nil, fmt.Errorf("count page objects: %w", err)
	}

	var figures []rawFigure
	for i := range countResp.Count {
		objResp, err := instance.FPDFPage_GetObject(
			&requests.FPDFPage_GetObject{Page: page, Index: i},
		)
		if err != nil {
			return nil, fmt.Errorf("get page object %d: %w", i, err)
		}

		typeResp, err := instance.FPDFPageObj_GetType(
			&requests.FPDFPageObj_GetType{PageObject: objResp.PageObject},
		)
		if err != nil {
			return nil, fmt.Errorf("get page object type %d: %w", i, err)
		}
		if typeResp.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}

		fig, ok, err := extractImageObject(instance, objResp.PageObject, pageArea)
		if err != nil {
			return nil, fmt.Errorf("extract image object %d: %w", i, err)
		}
		if ok {
			figures = append(figures, fig)
		}
	}
	return figures, nil
}

func extractImageObject(
	instance pdfium.Pdfium, obj references.FPDF_PAGEOBJECT, pageArea float64,
) (rawFigure, bool, error) {
	sizeResp, err := instance.FPDFImageObj_GetImagePixelSize(
		&requests.FPDFImageObj_GetImagePixelSize{ImageObject: obj},
	)
	if err != nil {
		return rawFigure{}, false, fmt.Errorf("get image pixel size: %w", err)
	}
	if sizeResp.Width < figureMinPixels || sizeResp.Height < figureMinPixels {
		return rawFigure{}, false, nil
	}

	boundsResp, err := instance.FPDFPageObj_GetBounds(
		&requests.FPDFPageObj_GetBounds{PageObject: obj},
	)
	if err != nil {
		return rawFigure{}, false, fmt.Errorf("get image bounds: %w", err)
	}
	left, bottom := float64(boundsResp.Left), float64(boundsResp.Bottom)
	right, top := float64(boundsResp.Right), float64(boundsResp.Top)
	area := (right - left) * (top - bottom)
	if pageArea <= 0 || area < figureMinAreaFraction*pageArea {
		return rawFigure{}, false, nil
	}

	pngData, err := renderImageObjectPNG(instance, obj)
	if err != nil {
		return rawFigure{}, false, err
	}

	return rawFigure{
		png: pngData, left: left, top: top, right: right, bottom: bottom,
	}, true, nil
}

func renderImageObjectPNG(
	instance pdfium.Pdfium, obj references.FPDF_PAGEOBJECT,
) ([]byte, error) {
	bitmapResp, err := instance.FPDFImageObj_GetBitmap(
		&requests.FPDFImageObj_GetBitmap{ImageObject: obj},
	)
	if err != nil {
		return nil, fmt.Errorf("get image bitmap: %w", err)
	}
	defer func() {
		_, _ = instance.FPDFBitmap_Destroy(
			&requests.FPDFBitmap_Destroy{Bitmap: bitmapResp.Bitmap},
		)
	}()

	img, err := bitmapToImage(instance, bitmapResp.Bitmap)
	if err != nil {
		return nil, fmt.Errorf("convert bitmap to image: %w", err)
	}

	var buf bytes.Buffer
	if err = png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// placeFigures resolves each raw figure's column/full-width placement,
// applies the document-level dedupe/cap via tracker, and writes accepted
// PNGs into workDir.
func placeFigures(
	raw []rawFigure, gutterLeft, gutterRight float64, twoColumn bool,
	tracker *figureTracker, workDir string,
) ([]pdfFigure, error) {
	gutterMid := (gutterLeft + gutterRight) / 2

	var figures []pdfFigure
	for _, r := range raw {
		name, ok := tracker.accept(r.png)
		if !ok {
			continue
		}
		if err := os.WriteFile(filepath.Join(workDir, name), r.png, 0o600); err != nil {
			return nil, fmt.Errorf("write figure %s: %w", name, err)
		}

		fig := pdfFigure{
			fileName: name, left: r.left, top: r.top, right: r.right, bottom: r.bottom,
		}
		if twoColumn && r.left < gutterMid && r.right > gutterMid {
			fig.fullWidth = true
		}
		figures = append(figures, fig)
	}
	return figures, nil
}

// renderFullPage rasterizes an image-only page (scanned content, or a page
// with too little extractable text and no surviving figures) to a single PNG
// at 150 DPI, per the image-only-page fallback.
func renderFullPage(
	instance pdfium.Pdfium, page requests.Page, index int, workDir string,
) (string, error) {
	renderResp, err := instance.RenderPageInDPI(
		&requests.RenderPageInDPI{Page: page, DPI: fullPageRenderDPI},
	)
	if err != nil {
		return "", fmt.Errorf("render full page: %w", err)
	}
	defer renderResp.Cleanup()

	var buf bytes.Buffer
	if err = png.Encode(&buf, renderResp.Result.Image); err != nil {
		return "", fmt.Errorf("encode full page png: %w", err)
	}

	name := fmt.Sprintf("fig-page-%d.png", index)
	if err = os.WriteFile(filepath.Join(workDir, name), buf.Bytes(), 0o600); err != nil {
		return "", fmt.Errorf("write full page png: %w", err)
	}
	return name, nil
}
