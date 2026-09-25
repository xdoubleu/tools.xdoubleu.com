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

// Figure filters: drop rules/bullets/glyph fragments by pixel size and tiny
// page coverage, and cap figures per document.
const (
	figureMinPixels       = 50
	figureMinAreaFraction = 0.01
	figureMaxPerDoc       = 50
	fullPageRenderDPI     = 150
	imageOnlyPageMaxChars = 200
)

//nolint:exhaustruct,gochecknoglobals // deliberately the zero value; read-only
var noFigure = rawFigure{}

// rawFigure has passed the per-object filters, not yet dedupe/cap.
type rawFigure struct {
	png                      []byte
	left, top, right, bottom float64
}

// pdfFigure is a placed figure already written to workDir.
type pdfFigure struct {
	fileName                 string
	left, top, right, bottom float64
	fullWidth                bool
}

//nolint:mnd // midpoint of a bounding box
func (f pdfFigure) xMid() float64 { return (f.left + f.right) / 2 }

// figureTracker dedupes figures by PNG SHA-256 and enforces the cap.
type figureTracker struct {
	seen  map[[32]byte]bool
	count int
}

func newFigureTracker() *figureTracker {
	return &figureTracker{seen: map[[32]byte]bool{}, count: 0}
}

// accept returns the filename and true for a first occurrence under the cap.
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

// extractPageImages keeps image objects passing the size and area filters.
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
		if typeResp.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}

		fig, ok, figErr := extractImageObject(instance, objResp.PageObject, pageArea)
		if figErr != nil {
			return nil, fmt.Errorf("extract image object %d: %w", i, figErr)
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
		return noFigure, false, fmt.Errorf("get image pixel size: %w", err)
	}
	if sizeResp.Width < figureMinPixels || sizeResp.Height < figureMinPixels {
		return noFigure, false, nil
	}

	boundsResp, err := instance.FPDFPageObj_GetBounds(
		&requests.FPDFPageObj_GetBounds{PageObject: obj},
	)
	if err != nil {
		return noFigure, false, fmt.Errorf("get image bounds: %w", err)
	}
	left, bottom := float64(boundsResp.Left), float64(boundsResp.Bottom)
	right, top := float64(boundsResp.Right), float64(boundsResp.Top)
	area := (right - left) * (top - bottom)
	if pageArea <= 0 || area < figureMinAreaFraction*pageArea {
		return noFigure, false, nil
	}

	pngData, err := renderImageObjectPNG(instance, obj)
	if err != nil {
		return noFigure, false, err
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

// placeFigures resolves placement, applies dedupe/cap, and writes PNGs.
func placeFigures(
	raw []rawFigure, gutterLeft, gutterRight float64, twoColumn bool,
	tracker *figureTracker, workDir string,
) ([]pdfFigure, error) {
	gutterMid := (gutterLeft + gutterRight) / midpointDivisor

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
			fileName:  name,
			left:      r.left,
			top:       r.top,
			right:     r.right,
			bottom:    r.bottom,
			fullWidth: twoColumn && r.left < gutterMid && r.right > gutterMid,
		}
		figures = append(figures, fig)
	}
	return figures, nil
}

// renderFullPage rasterizes an image-only page at 150 DPI through the same
// figureTracker, so duplicate (e.g. blank) pages dedupe and the cap applies.
// An empty name with nil error means the tracker rejected it.
func renderFullPage(
	instance pdfium.Pdfium, page requests.Page, workDir string, tracker *figureTracker,
) (string, error) {
	renderResp, err := instance.RenderPageInDPI(
		&requests.RenderPageInDPI{ //nolint:exhaustruct // defaults suit a plain page render
			Page: page,
			DPI:  fullPageRenderDPI,
		},
	)
	if err != nil {
		return "", fmt.Errorf("render full page: %w", err)
	}
	defer renderResp.Cleanup()

	var buf bytes.Buffer
	if err = png.Encode(&buf, renderResp.Result.RenderedImage); err != nil {
		return "", fmt.Errorf("encode full page png: %w", err)
	}

	name, ok := tracker.accept(buf.Bytes())
	if !ok {
		return "", nil
	}

	if err = os.WriteFile(filepath.Join(workDir, name), buf.Bytes(), 0o600); err != nil {
		return "", fmt.Errorf("write full page png: %w", err)
	}
	return name, nil
}
