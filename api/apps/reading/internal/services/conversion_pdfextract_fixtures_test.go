package services

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/require"
)

// Fixture layout constants (all in PDF points, unit "pt"). Chosen with wide
// safety margins so paragraph-break/heading/gutter classification is robust
// to exact glyph-metric variance rather than depending on precise
// hand-computed font measurements.
const (
	fixturePageWidth  = 700.0
	fixturePageHeight = 800.0

	fixtureLeftX     = 40.0
	fixtureGutterEnd = 380.0
	fixtureRightX    = 380.0

	fixtureBodySize    = 10.0
	fixtureHeadingSize = 20.0

	// fixtureSameParaDY is the baseline-to-baseline gap used between two
	// lines that must join into one paragraph (the hyphenation pair).
	fixtureSameParaDY = 14.0
	// fixtureBreakDY is the baseline-to-baseline gap used between two lines
	// that must start separate paragraphs.
	fixtureBreakDY = 70.0
)

// newFixturePDF returns a blank landscape-agnostic custom-size PDF ready for
// hand-placed text/images, with unit "pt" so positions map 1:1 to PDF points.
func newFixturePDF() *fpdf.Fpdf {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		SizeStr:        "",
		Size:           fpdf.SizeType{Wd: fixturePageWidth, Ht: fixturePageHeight},
		FontDirStr:     "",
	})
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(0, 0, 0)
	pdf.AddPage()
	return pdf
}

func fixtureText(pdf *fpdf.Fpdf, x, y, size float64, s string) {
	pdf.SetFont("Helvetica", "", size)
	pdf.Text(x, y, s)
}

// solidPNG generates a small solid-color PNG in-code (no binary fixture is
// committed): a plain filled rectangle is enough to exercise bitmap
// extraction/encoding without needing real image content.
func solidPNG(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func fixtureImage(
	t *testing.T, pdf *fpdf.Fpdf, name string, x, y, w, h float64, c color.Color,
) {
	t.Helper()
	data := solidPNG(t, 120, 100, c)
	pdf.RegisterImageOptionsReader(
		name,
		fpdf.ImageOptions{ImageType: "PNG"},
		bytes.NewReader(data),
	)
	pdf.ImageOptions(
		name,
		x,
		y,
		w,
		h,
		false,
		fpdf.ImageOptions{ImageType: "PNG"},
		0,
		"",
	)
}

func savePDF(t *testing.T, pdf *fpdf.Fpdf, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, pdf.OutputFileAndClose(path))
	return path
}

const (
	// twoColHeading is deliberately short: the column-right-edge stat used by
	// the short-line paragraph-break rule is a max over every line in the
	// column, including headings, and body lines here are single, unwrapped
	// strings of varying length rather than text actually wrapped to fill the
	// column — a wide heading would dominate that max and make every
	// legitimately-continuing body line look "short" by comparison.
	twoColHeading     = "HEADING"
	twoColParaA       = "Left paragraph of body text content here"
	twoColParaBL1     = "This word right here becomes assess-"
	twoColParaBL2     = "ment and it continues onward nicely"
	twoColParaBJoined = "This word right here becomes assessment and it continues onward nicely"
	twoColParaC       = "Final paragraph closes out the column now"
	twoColParaD       = "Right column paragraph opens the second column nicely"
	twoColParaE       = "Second column paragraph wraps up this page for good"
	twoColFigName     = "fixture-two-col-fig.png"
)

// makeTwoColumnPDF builds a single two-column page: a heading, a figure
// placed between two known paragraphs in the left column, a hyphenated word
// split across two lines, and a short paragraph closing the column, followed
// by two paragraphs in the right column. The right column's lines are placed
// at Y offsets that never coincide with the left column's, since step 1 (line
// grouping) clusters purely by y-midpoint proximity — real two-column layouts
// rarely align their row grids exactly across the whole page, and this
// fixture must not accidentally do so either, or lines from both columns
// would be grouped into one before column splitting ever runs.
func makeTwoColumnPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()

	fixtureText(pdf, fixtureLeftX, 80, fixtureHeadingSize, twoColHeading)
	fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, twoColParaA)

	figY := 80 + fixtureBreakDY + 30
	fixtureImage(
		t,
		pdf,
		twoColFigName,
		fixtureLeftX,
		figY,
		100,
		80,
		color.RGBA{R: 200, G: 80, B: 80, A: 255},
	)

	paraBY := figY + 80 + 30
	fixtureText(pdf, fixtureLeftX, paraBY, fixtureBodySize, twoColParaBL1)
	fixtureText(
		pdf,
		fixtureLeftX,
		paraBY+fixtureSameParaDY,
		fixtureBodySize,
		twoColParaBL2,
	)

	fixtureText(
		pdf,
		fixtureLeftX,
		paraBY+fixtureSameParaDY+fixtureBreakDY,
		fixtureBodySize,
		twoColParaC,
	)

	fixtureText(pdf, fixtureRightX, 97, fixtureBodySize, twoColParaD)
	fixtureText(pdf, fixtureRightX, 97+fixtureBreakDY, fixtureBodySize, twoColParaE)

	return savePDF(t, pdf, "two-column.pdf")
}

const (
	singleColHeading = "HEADING"
	singleColParaA   = "Opening paragraph of the single column body text goes here"
	singleColParaB   = "Second paragraph of the single column body text follows after a gap"
	// singleColParaC pads the page's total extractable-character count past
	// the 200-character image-only-page threshold — without it this fixture
	// would (correctly, per that rule) get rendered as a full-page fallback
	// image instead of exercising the paragraph/heading path this test wants.
	singleColParaC = "Extra closing paragraph of body text adds more length for this section now and then some more words follow"
)

// makeSingleColumnPDF builds a single-column page with one heading and three
// body paragraphs.
func makeSingleColumnPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()

	fixtureText(pdf, fixtureLeftX, 80, fixtureHeadingSize, singleColHeading)
	fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, singleColParaA)
	fixtureText(
		pdf,
		fixtureLeftX,
		80+fixtureBreakDY+fixtureBreakDY,
		fixtureBodySize,
		singleColParaB,
	)
	fixtureText(
		pdf, fixtureLeftX, 80+3*fixtureBreakDY, fixtureBodySize, singleColParaC,
	)

	return savePDF(t, pdf, "single-column.pdf")
}

// makeImageOnlyPDF builds a page containing only an image, sized under the
// 1%-of-page-area figure filter so it is never extracted as a standalone
// figure — exercising the "no images survived the filters" branch of the
// image-only-page fallback, which rasterizes the whole page instead. A real
// scanned page's raster is normally far larger than this, but what the
// fallback branch keys on is exactly this condition (near-zero text, zero
// surviving figures), not the image's absolute size.
func makeImageOnlyPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureImage(
		t, pdf, "fixture-image-only.png", 100, 100, 20, 20,
		color.RGBA{R: 60, G: 120, B: 200, A: 255},
	)
	return savePDF(t, pdf, "image-only.pdf")
}

// makeLogoRepeatedPDF places the same small image on three pages (to
// exercise document-level SHA-256 dedupe) alongside a tiny sub-50px image (to
// exercise the pixel-size filter) and enough body text that these pages
// don't trip the image-only-page fallback.
func makeLogoRepeatedPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()

	logo := solidPNG(t, 80, 80, color.RGBA{R: 10, G: 200, B: 10, A: 255})
	tiny := solidPNG(t, 20, 20, color.RGBA{R: 10, G: 10, B: 200, A: 255})

	for page := range 3 {
		if page > 0 {
			pdf.AddPage()
		}
		fixtureText(pdf, fixtureLeftX, 80, fixtureBodySize,
			"Body text on this page keeps it from being treated as image-only content")
		fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize,
			"A second short paragraph adds a bit more extractable text as well")

		logoName := "logo-page.png"
		pdf.RegisterImageOptionsReader(
			logoName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(logo),
		)
		pdf.ImageOptions(
			logoName, fixtureLeftX, 300, 80, 80, false,
			fpdf.ImageOptions{ImageType: "PNG"}, 0, "",
		)

		tinyName := "tiny-page.png"
		pdf.RegisterImageOptionsReader(
			tinyName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(tiny),
		)
		pdf.ImageOptions(
			tinyName, fixtureLeftX+150, 300, 20, 20, false,
			fpdf.ImageOptions{ImageType: "PNG"}, 0, "",
		)
	}

	return savePDF(t, pdf, "logo-repeated.pdf")
}
