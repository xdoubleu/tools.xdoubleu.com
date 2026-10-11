//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
