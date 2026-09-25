//nolint:testpackage // testing unexported service helpers
package services

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/require"
)

// Fixture layout in PDF points, with wide margins so classification doesn't
// depend on exact glyph metrics.
const (
	fixturePageWidth  = 700.0
	fixturePageHeight = 800.0

	fixtureLeftX     = 40.0
	fixtureGutterEnd = 380.0
	fixtureRightX    = 380.0

	fixtureBodySize    = 10.0
	fixtureHeadingSize = 20.0

	fixtureSameParaDY = 14.0
	fixtureBreakDY    = 70.0
)

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

//nolint:gochecknoglobals // read-only shared test fixture config
var pngImageOptions = fpdf.ImageOptions{
	ImageType:             "PNG",
	ReadDpi:               false,
	AllowNegativePosition: false,
}

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
	pdf.RegisterImageOptionsReader(name, pngImageOptions, bytes.NewReader(data))
	pdf.ImageOptions(name, x, y, w, h, false, pngImageOptions, 0, "")
}

func savePDF(t *testing.T, pdf *fpdf.Fpdf, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, pdf.OutputFileAndClose(path))
	return path
}

const (
	// twoColHeading is short because the column right edge is a max over every
	// line; a wide heading would make unwrapped body lines all look "short".
	twoColHeading     = "HEADING"
	twoColParaA       = "Left paragraph of body text content here"
	twoColParaBL1     = "This word right here becomes assess-"
	twoColParaBL2     = "ment and it continues onward nicely"
	twoColParaBJoined = "This word right here becomes assessment and it " +
		"continues onward nicely"
	twoColParaC   = "Final paragraph closes out the column now"
	twoColParaD   = "Right column paragraph opens the second column nicely"
	twoColParaE   = "Second column paragraph wraps up this page for good"
	twoColFigName = "fixture-two-col-fig.png"
)

// makeTwoColumnPDF builds a two-column page: heading, a figure between left
// paragraphs, a hyphenated split, and two right-column paragraphs. Right
// column Y offsets never coincide with the left's, or line grouping would
// merge them before column splitting.
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
	singleColParaB   = "Second paragraph of the single column body text follows " +
		"after a gap"
	// singleColParaC pushes the page past the 200-char image-only threshold.
	singleColParaC = "Extra closing paragraph of body text adds more length " +
		"for this section now and then some more words follow"
)

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

// makeImageOnlyPDF has one image under the 1%-of-page figure filter, so the
// image-only fallback rasterizes the whole page.
func makeImageOnlyPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureImage(
		t, pdf, "fixture-image-only.png", 100, 100, 20, 20,
		color.RGBA{R: 60, G: 120, B: 200, A: 255},
	)
	return savePDF(t, pdf, "image-only.pdf")
}

// makeLogoRepeatedPDF repeats one image on three pages (dedupe) plus a
// sub-50px image (size filter), with enough text to avoid the fallback.
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
		pdf.RegisterImageOptionsReader(logoName, pngImageOptions, bytes.NewReader(logo))
		pdf.ImageOptions(
			logoName,
			fixtureLeftX,
			300,
			80,
			80,
			false,
			pngImageOptions,
			0,
			"",
		)

		tinyName := "tiny-page.png"
		pdf.RegisterImageOptionsReader(tinyName, pngImageOptions, bytes.NewReader(tiny))
		pdf.ImageOptions(
			tinyName, fixtureLeftX+150, 300, 20, 20, false, pngImageOptions, 0, "",
		)
	}

	return savePDF(t, pdf, "logo-repeated.pdf")
}

// makeImageOnlyMultiPagePDF builds five image-only-fallback pages: four blank
// (identical rasters) and one vector-filled, to exercise full-page raster
// dedupe.
func makeImageOnlyMultiPagePDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()

	pdf.AddPage()
	pdf.AddPage()

	pdf.AddPage()
	pdf.SetFillColor(200, 60, 60)
	pdf.Rect(0, 0, fixturePageWidth, fixturePageHeight, "F")

	pdf.AddPage()

	return savePDF(t, pdf, "image-only-multi-page.pdf")
}

// proofSlugPageCount clears removeProofSlugLines' recurrence threshold.
const proofSlugPageCount = 5

const (
	proofSlugBodyParaFmt = "Body paragraph %d discusses systems thinking " +
		"concepts and feedback loops in some extra detail here"
	proofSlugBodyExtraFmt = "Second paragraph %d continues the discussion " +
		"of stocks and flows within the same system boundary"
	// proofSlugFooterFmt has the proof-slug shape (page number, date, time) with
	// no book-specific text. Its words use only x-height letters: fpdf gives
	// ascenders/digits taller boxes, which would otherwise make the line classify
	// as a heading in this synthetic fixture.
	proofSlugFooterFmt = "canoe scene ocean scan %d sonar %s arena %s"
	// proofSlugDateInBodyPara has a date but no page number or time, so it must
	// never be treated as a slug.
	proofSlugDateInBodyPara = "The revised schedule set the deadline for " +
		"5/2/09 according to the committee notes"
	// proofSlugBodyClosingFmt pushes each page past the 200-char image-only
	// threshold.
	proofSlugBodyClosingFmt = "Closing paragraph %d wraps up the page with " +
		"a bit more body text so the page is never treated as image-only content"
)

// makeProofSlugPDF builds pages with three body paragraphs and a bottom proof
// slug; the last page swaps in proofSlugDateInBodyPara.
func makeProofSlugPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()

	// No digit "1": fpdf misplaces the gap beside it enough to split tokens.
	pageNums := []int{72, 73, 74, 75, 76}
	dates := []string{"5/2/09", "5/3/09", "5/4/09", "5/5/09", "5/6/09"}
	times := []string{
		"9:37:22", "9:37:24", "9:37:26", "9:37:28", "9:37:32",
	}

	for page := range proofSlugPageCount {
		if page > 0 {
			pdf.AddPage()
		}
		fixtureText(pdf, fixtureLeftX, 80, fixtureBodySize,
			fmt.Sprintf(proofSlugBodyParaFmt, page+1))

		secondPara := fmt.Sprintf(proofSlugBodyExtraFmt, page+1)
		if page == proofSlugPageCount-1 {
			secondPara = proofSlugDateInBodyPara
		}
		fixtureText(
			pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, secondPara,
		)
		fixtureText(
			pdf, fixtureLeftX, 80+2*fixtureBreakDY, fixtureBodySize,
			fmt.Sprintf(proofSlugBodyClosingFmt, page+1),
		)

		fixtureText(pdf, fixtureLeftX, 760, fixtureBodySize,
			fmt.Sprintf(
				proofSlugFooterFmt, pageNums[page], dates[page], times[page],
			))
	}

	return savePDF(t, pdf, "proof-slug.pdf")
}
