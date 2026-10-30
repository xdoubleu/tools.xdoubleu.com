//nolint:testpackage // testing unexported span-map helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/services/conversion"
)

// PDF fixture layout in points (see the conversion package's
// conversion_pdfextract_fixtures_test.go for the full set).
const (
	fixturePageWidth  = 700.0
	fixturePageHeight = 800.0

	fixtureLeftX       = 40.0
	fixtureBodySize    = 10.0
	fixtureBreakDY     = 70.0
	fixtureHeadingSize = 20.0
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

func fixtureText(pdf *fpdf.Fpdf, y, size float64, s string) {
	pdf.SetFont("Helvetica", "", size)
	pdf.Text(fixtureLeftX, y, s)
}

func savePDF(t *testing.T, pdf *fpdf.Fpdf, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, pdf.OutputFileAndClose(path))
	return path
}

const (
	pagedHeading   = "Opening Chapter"
	pagedParaOne   = "First page paragraph opens the book here."
	pagedParaTwo   = "Second page paragraph carries on with the story."
	pagedParaTwoB  = "Another paragraph sits lower on page two and"
	pagedParaThree = "continues onto page three, closing the tale."
)

// pagedTexts is each anchored page's first text.
//
//nolint:gochecknoglobals // read-only fixture table
var pagedTexts = map[int]string{1: pagedHeading, 2: pagedParaTwo, 3: pagedParaThree}

// makePagedPDF has text on pages 1–3, page 2's last paragraph continuing on
// page 3, and a blank page 4.
func makePagedPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureText(pdf, 80, fixtureHeadingSize, pagedHeading)
	fixtureText(pdf, 80+fixtureBreakDY, fixtureBodySize, pagedParaOne)
	pdf.AddPage()
	fixtureText(pdf, 80, fixtureBodySize, pagedParaTwo)
	fixtureText(pdf, 80+fixtureBreakDY, fixtureBodySize, pagedParaTwoB)
	pdf.AddPage()
	fixtureText(pdf, 80, fixtureBodySize, pagedParaThree)
	pdf.AddPage()
	return savePDF(t, pdf, "paged.pdf")
}

// convertToEPUB runs the conversion package's PDF converter with no catalog
// metadata and returns the EPUB path.
func convertToEPUB(t *testing.T, pdfPath string) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.epub")
	err := conversion.GoPDFConverter(
		context.Background(), pdfPath, outPath,
		"00000000-0000-0000-0000-000000000001", "", nil,
	)
	require.NoError(t, err)
	return outPath
}

// pagedKEPUB converts makePagedPDF and runs the result through kepubify.
func pagedKEPUB(t *testing.T) (string, *zip.Reader) {
	t.Helper()
	epubPath := convertToEPUB(t, makePagedPDF(t))
	data, err := os.ReadFile(epubPath)
	require.NoError(t, err)
	kepub, err := conversion.NewKepubifyConverter().Convert(context.Background(), data)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(kepub), int64(len(kepub)))
	require.NoError(t, err)
	return epubPath, zr
}

func TestPDFSourcedSpanMap_PageRoundTrip(t *testing.T) {
	t.Parallel()
	_, kepub := pagedKEPUB(t)

	m, err := buildSpanMap(context.Background(), kepub, nil)
	require.NoError(t, err)
	require.True(t, m.pdf)
	require.Len(t, m.pages, 3)

	for page, text := range pagedTexts {
		loc := m.location(pagePos(page))
		require.NotNil(t, loc, "page %d", page)
		dt, docErr := zipDocText(kepub, loc.Source)
		require.NoError(t, docErr)
		assert.True(t, strings.HasPrefix(spanText(t, dt, loc.Value), text),
			"page %d lands on %q", page, text)

		pos := m.position(*loc)
		require.NotNil(t, pos)
		assert.Equal(t, page, pos.Page, "span → page round-trips")
	}

	blank := m.location(pagePos(4))
	assert.Equal(t, m.location(pagePos(3)), blank,
		"a page without text maps to the nearest page before it")
}

// spanText is a koboSpan's text in dt.
func spanText(t *testing.T, dt docText, value string) string {
	t.Helper()
	id, ok := parseSpanID(value)
	require.True(t, ok)
	for _, s := range dt.spans {
		if s.id == id {
			return str(dt.units[s.start:])
		}
	}
	t.Fatalf("span %s not found", value)
	return ""
}
