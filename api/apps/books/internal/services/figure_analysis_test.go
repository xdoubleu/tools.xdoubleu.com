//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func TestCountPDFFigures_VectorFigure(t *testing.T) {
	stats, err := countPDFFigures(context.Background(), makeVectorFigurePDF(t))
	require.NoError(t, err)
	assert.Equal(t, models.FigureStats{Figures: 1, Pages: 1}, stats)
}

func TestCountPDFFigures_TextOnly(t *testing.T) {
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "Just prose on the first page.")
	pdf.AddPage()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "And more on the second.")

	stats, err := countPDFFigures(context.Background(), savePDF(t, pdf, "text.pdf"))
	require.NoError(t, err)
	assert.Equal(t, models.FigureStats{Figures: 0, Pages: 2}, stats)
}

// Two figures beside text count; a page-sized image under text (a scan's
// image layer) and a tiny one don't; a textless scanned page does.
func TestCountPDFFigures_RasterImages(t *testing.T) {
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "A page with two figures.")
	fixtureImage(t, pdf, "a", 100, 100, 240, 200, color.RGBA{R: 200, G: 0, B: 0, A: 255})
	fixtureImage(t, pdf, "b", 100, 400, 240, 200, color.RGBA{R: 0, G: 200, B: 0, A: 255})
	fixtureImage(t, pdf, "tiny", 400, 100, 5, 5, color.RGBA{R: 0, G: 0, B: 200, A: 255})

	pdf.AddPage()
	fixtureImage(t, pdf, "layer", 0, 0, fixturePageWidth, fixturePageHeight,
		color.RGBA{R: 250, G: 250, B: 250, A: 255})
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "OCR text over the scan.")

	pdf.AddPage()
	fixtureImage(t, pdf, "scan", 0, 0, fixturePageWidth, fixturePageHeight,
		color.RGBA{R: 10, G: 10, B: 10, A: 255})

	stats, err := countPDFFigures(context.Background(), savePDF(t, pdf, "images.pdf"))
	require.NoError(t, err)
	assert.Equal(t, models.FigureStats{Figures: 3, Pages: 3}, stats)
}

func TestCountPDFFigures_NotAPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pdf")
	require.NoError(t, os.WriteFile(path, []byte("not a pdf"), 0o600))

	_, err := countPDFFigures(context.Background(), path)
	require.Error(t, err)
}

func TestCountPDFFigures_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := countPDFFigures(ctx, makeVectorFigurePDF(t))
	require.ErrorIs(t, err, context.Canceled)
}

// imageFigureBounds keeps only image objects that pass the pixel-size and
// page-area filters, and never carries pixel data.
func TestImageFigureBounds_Filters(t *testing.T) {
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "Figures on this page.")
	fixtureImage(t, pdf, "fig", 100, 400, 240, 200, color.RGBA{R: 200, G: 0, B: 0, A: 255})
	// Under the 1% page-area floor.
	fixtureImage(
		t,
		pdf,
		"tiny-area",
		400,
		100,
		5,
		5,
		color.RGBA{R: 0, G: 0, B: 200, A: 255},
	)
	// Under the 50px pixel-size floor.
	tiny := solidPNG(t, 40, 40, color.White)
	pdf.RegisterImageOptionsReader("small", pngImageOptions, bytes.NewReader(tiny))
	pdf.ImageOptions("small", 200, 400, 48, 40, false, pngImageOptions, 0, "")
	path := savePDF(t, pdf, "filter.pdf")

	err := withPDFDocument(context.Background(), path,
		func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error {
			page := requests.Page{ //nolint:exhaustruct // by index
				ByIndex: &requests.PageByIndex{Document: doc, Index: 0},
			}
			figures, err := imageFigureBounds(instance, page,
				fixturePageWidth*fixturePageHeight)
			require.NoError(t, err)
			require.Len(t, figures, 1)
			assert.Nil(t, figures[0].png)
			return nil
		})
	require.NoError(t, err)
}

// vectorFigureBounds yields a vector diagram's region bounds (png nil) and
// leaves the label chars out, mirroring the converter without rendering.
func TestVectorFigureBounds_NoRender(t *testing.T) {
	err := withPDFDocument(context.Background(), makeVectorFigurePDF(t),
		func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error {
			page := requests.Page{ //nolint:exhaustruct // by index
				ByIndex: &requests.PageByIndex{Document: doc, Index: 0},
			}
			geo, chars, err := pageText(instance, page)
			require.NoError(t, err)

			figures, rest, err := vectorFigureBounds(instance, page, geo, chars)
			require.NoError(t, err)
			require.NotEmpty(t, figures, "the diagram must count as a figure")
			for i := range figures {
				assert.Nil(t, figures[i].png)
			}
			joined := ""
			for _, c := range rest {
				joined += c.text
			}
			assert.NotContains(t, joined, "stock")
			return nil
		})
	require.NoError(t, err)
}

// A text-only page has no vector figure regions.
func TestVectorFigureBounds_NoBoxes(t *testing.T) {
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "Just prose.")
	path := savePDF(t, pdf, "text.pdf")

	err := withPDFDocument(context.Background(), path,
		func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error {
			page := requests.Page{ //nolint:exhaustruct // by index
				ByIndex: &requests.PageByIndex{Document: doc, Index: 0},
			}
			geo, chars, err := pageText(instance, page)
			require.NoError(t, err)
			figures, _, err := vectorFigureBounds(instance, page, geo, chars)
			require.NoError(t, err)
			assert.Empty(t, figures)
			return nil
		})
	require.NoError(t, err)
}

// The counting path must not render figure pixels: a huge embedded image
// would balloon the pdfium wasm heap just to be counted (the api restart
// loop). Bounds-only figures keep the same count while skipping GetBitmap
// and the page render.
func TestCountPageFigures_CountingSkipsRendering(t *testing.T) {
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, "A page with a figure.")
	fixtureImage(t, pdf, "a", 100, 400, 240, 200, color.RGBA{R: 200, G: 0, B: 0, A: 255})
	path := savePDF(t, pdf, "render.pdf")

	err := withPDFDocument(context.Background(), path,
		func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error {
			page := requests.Page{ //nolint:exhaustruct // by index
				ByIndex: &requests.PageByIndex{Document: doc, Index: 0},
			}
			geo, chars, err := pageText(instance, page)
			require.NoError(t, err)

			counted, err := figureBounds(instance, page, geo, chars)
			require.NoError(t, err)
			require.NotEmpty(t, counted, "the image must count as a figure")
			for i := range counted {
				assert.Nil(t, counted[i].png, "counting must not render pixels")
			}

			// The converter's pageFigures must still render them.
			rendered, _, err := pageFigures(instance, page, geo, chars)
			require.NoError(t, err)
			require.Len(t, rendered, len(counted))
			for i := range rendered {
				assert.NotEmpty(t, rendered[i].png, "conversion still renders pixels")
			}
			return nil
		})
	require.NoError(t, err)
}

func figureTestJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 80, 80)), nil))
	return buf.Bytes()
}

// figureTestEPUB's figures: big.png (shown twice), photo.jpg, chart.svg, an
// inline drawing, and an inline SVG wrapping inner.png. Not figures: the
// cover, small.png, an undecodable image, a sourceless and a remote one.
func figureTestEPUB(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, data []byte) {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}

	write("META-INF/container.xml", []byte(`<?xml version="1.0"?>`+
		`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"`+
		` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"`+
		` media-type="application/oebps-package+xml"/></rootfiles></container>`))
	write("OEBPS/content.opf", []byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
<manifest>
<item id="cover" href="img/cover.png" media-type="image/png" properties="cover-image"/>
<item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="ch2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
<item id="gone" href="text/gone.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="ch1"/><itemref idref="unknown"/><itemref idref="gone"/>`+
		`<itemref idref="ch2"/></spine>
</package>`))

	big := solidPNG(t, 120, 100, color.White)
	write("OEBPS/img/cover.png", big)
	write("OEBPS/img/big.png", big)
	write("OEBPS/img/inner.png", big)
	write("OEBPS/img/small.png", solidPNG(t, 20, 20, color.White))
	write("OEBPS/img/photo.jpg", figureTestJPEG(t))
	write("OEBPS/img/chart.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	write("OEBPS/img/broken.png", []byte("not an image"))

	write("OEBPS/text/ch1.xhtml", []byte(`<html><head><title>One</title>`+
		`<img src="../img/photo.jpg"/></head><body>`+
		`<img src="../img/cover.png"/>`+
		`<p>`+strings.Repeat("a", 3000)+`</p>`+
		`<img src="../img/big.png#frag"/><img src="../img/small.png"/>`+
		`<img src="https://example.com/remote.png"/><img/>`+
		`<img src="../img/broken.png"/>`+
		`<svg><path d="M0 0L10 10"/></svg></body></html>`))
	write("OEBPS/text/ch2.xhtml", []byte(`<html><body>`+
		`<p>`+strings.Repeat("b ", 1500)+`</p>`+
		`<img src="../img/big.png"/><img src="../img/photo.jpg"/>`+
		`<img src="../img/chart.svg"/>`+
		`<svg><g><image xlink:href="../img/inner.png"/></g></svg></body></html>`))
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestCountEPUBFigures(t *testing.T) {
	data := figureTestEPUB(t)

	stats, err := countEPUBFigures(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	// 4500 characters of text: three pages.
	assert.Equal(t, models.FigureStats{Figures: 5, Pages: 3}, stats)
}

func TestCountEPUBFigures_NoText(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("META-INF/container.xml")
	require.NoError(t, err)
	_, err = w.Write([]byte(`<container><rootfiles><rootfile full-path="p.opf"` +
		` media-type="application/oebps-package+xml"/></rootfiles></container>`))
	require.NoError(t, err)
	w, err = zw.Create("p.opf")
	require.NoError(t, err)
	_, err = w.Write([]byte(`<package><manifest/><spine/></package>`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	stats, err := countEPUBFigures(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	assert.Equal(t, models.FigureStats{Figures: 0, Pages: 1}, stats)
}

func TestCountEPUBFigures_Invalid(t *testing.T) {
	_, err := countEPUBFigures(strings.NewReader("not a zip"), 9)
	require.Error(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	require.NoError(t, zw.Close())
	_, err = countEPUBFigures(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.Error(t, err, "no container.xml")

	buf.Reset()
	zw = zip.NewWriter(&buf)
	w, err := zw.Create("META-INF/container.xml")
	require.NoError(t, err)
	_, err = w.Write([]byte(`<container><rootfiles><rootfile full-path="p.opf"` +
		` media-type="application/oebps-package+xml"/></rootfiles></container>`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	_, err = countEPUBFigures(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.Error(t, err, "no package document")
}

// errFigureCall is the error every failing fake pdfium call returns.
var errFigureCall = errors.New("pdfium call failed")

// failPDFium fakes the pdfium calls the bounds helpers make. failOn names a
// method whose nth call (1-based) errors; other calls succeed with
// sane-enough defaults (one IMAGE object, a big pixel size, page bounds) so
// earlier steps pass until the targeted call. It embeds the real interface so
// any unexpected call panics instead of silently passing.
type failPDFium struct {
	pdfium.Pdfium
	failOn map[string]int
	calls  map[string]int
}

func (f *failPDFium) next(name string) error {
	f.calls[name]++
	if f.failOn[name] == f.calls[name] {
		return errFigureCall
	}
	return nil
}

//nolint:revive // names must match the pdfium.Pdfium interface
func (f *failPDFium) FPDFPage_CountObjects(
	*requests.FPDFPage_CountObjects,
) (*responses.FPDFPage_CountObjects, error) {
	if err := f.next("CountObjects"); err != nil {
		return nil, err
	}
	return &responses.FPDFPage_CountObjects{Count: 1}, nil
}

//nolint:revive // names must match the pdfium.Pdfium interface
func (f *failPDFium) FPDFPage_GetObject(
	*requests.FPDFPage_GetObject,
) (*responses.FPDFPage_GetObject, error) {
	if err := f.next("GetObject"); err != nil {
		return nil, err
	}
	return &responses.FPDFPage_GetObject{
		PageObject: "",
	}, nil
}

//nolint:revive // names must match the pdfium.Pdfium interface
func (f *failPDFium) FPDFPageObj_GetType(
	*requests.FPDFPageObj_GetType,
) (*responses.FPDFPageObj_GetType, error) {
	if err := f.next("GetType"); err != nil {
		return nil, err
	}
	return &responses.FPDFPageObj_GetType{Type: enums.FPDF_PAGEOBJ_IMAGE}, nil
}

//nolint:revive // names must match the pdfium.Pdfium interface
func (f *failPDFium) FPDFImageObj_GetImagePixelSize(
	*requests.FPDFImageObj_GetImagePixelSize,
) (*responses.FPDFImageObj_GetImagePixelSize, error) {
	if err := f.next("PixelSize"); err != nil {
		return nil, err
	}
	return &responses.FPDFImageObj_GetImagePixelSize{Width: 1000, Height: 1000}, nil
}

//nolint:revive // names must match the pdfium.Pdfium interface
func (f *failPDFium) FPDFPageObj_GetBounds(
	*requests.FPDFPageObj_GetBounds,
) (*responses.FPDFPageObj_GetBounds, error) {
	if err := f.next("Bounds"); err != nil {
		return nil, err
	}
	return &responses.FPDFPageObj_GetBounds{Left: 0, Bottom: 0, Right: 100, Top: 100}, nil
}

func newFailPDFium(method string, on int) *failPDFium {
	return &failPDFium{
		Pdfium: pdfium.Pdfium(nil),
		failOn: map[string]int{method: on},
		calls:  make(map[string]int),
	}
}

// Every pdfium failure in the image-bounds path propagates as an error.
func TestImageFigureBounds_PropagatesErrors(t *testing.T) {
	page := requests.Page{ //nolint:exhaustruct // by index
		ByIndex: &requests.PageByIndex{Document: "doc", Index: 0},
	}
	for _, method := range []string{
		"CountObjects", "GetObject", "GetType", "PixelSize", "Bounds",
	} {
		t.Run(method, func(t *testing.T) {
			_, err := imageFigureBounds(newFailPDFium(method, 1), page, 100_000)
			require.ErrorIs(t, err, errFigureCall)
		})
	}
}

// figureBounds propagates errors from each half of the page.
func TestFigureBounds_PropagatesErrors(t *testing.T) {
	page := requests.Page{ //nolint:exhaustruct // by index
		ByIndex: &requests.PageByIndex{Document: "doc", Index: 0},
	}
	geo := pageGeometry{originX: 0, originY: 0, width: 100, height: 100}

	_, err := figureBounds(newFailPDFium("CountObjects", 1), page, geo, nil)
	require.ErrorIs(t, err, errFigureCall)

	// The second CountObjects call is pathBoxes inside vectorFigureBounds,
	// after imageFigureBounds has already succeeded.
	_, err = figureBounds(newFailPDFium("CountObjects", 2), page, geo, nil)
	require.ErrorIs(t, err, errFigureCall)
}

// vectorFigureBounds propagates a pathBoxes failure.
func TestVectorFigureBounds_PropagatesPathError(t *testing.T) {
	page := requests.Page{ //nolint:exhaustruct // by index
		ByIndex: &requests.PageByIndex{Document: "doc", Index: 0},
	}
	_, _, err := vectorFigureBounds(newFailPDFium("CountObjects", 1), page,
		pageGeometry{originX: 0, originY: 0, width: 100, height: 100}, nil)
	require.ErrorIs(t, err, errFigureCall)
}
