//nolint:testpackage // testing unexported service helpers
package services

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/require"
)

// Index-page geometry from the reference book: a 433pt-wide trim box offset
// 85pt into a larger MediaBox, with two columns 12pt (2.8%) apart.
const (
	indexTrimWidth  = 433.0
	indexTrimHeight = 660.0
	indexCropOffset = 85.0
	indexLeftX      = 54.0
	indexRightX     = 222.0
	indexColWidth   = 156.0
	indexLineDY     = 13.0
	indexRows       = 30
)

// makeCroppedIndexPDF draws a two-column index shifted indexCropOffset into a
// wider page, then declares a CropBox that trims the offset back off.
func makeCroppedIndexPDF(t *testing.T) string {
	t.Helper()
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		SizeStr:        "",
		Size: fpdf.SizeType{
			Wd: indexTrimWidth + indexCropOffset, Ht: indexTrimHeight,
		},
		FontDirStr: "",
	})
	pdf.SetCompression(false)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(0, 0, 0)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 9)

	// The title straddles the gutter, above both columns.
	pdf.SetFont("Helvetica", "", 20)
	pdf.Text(indexCropOffset+190, 90, "Index")
	pdf.SetFont("Helvetica", "", 9)

	// A proof slug in the trimmed-off margin is invisible on the page.
	pdf.Text(10, indexTrimHeight-20, "hidden slug text outside the crop")

	for row := range indexRows {
		y := 180 + float64(row)*indexLineDY
		left := padToWidth(pdf, fmt.Sprintf("left entry %c term, %d", 'a'+row%26, row), indexColWidth)
		right := padToWidth(pdf, fmt.Sprintf("right entry %c term, %d", 'a'+row%26, row), indexColWidth)
		pdf.Text(indexCropOffset+indexLeftX, y, left)
		pdf.Text(indexCropOffset+indexRightX, y, right)
	}

	var buf bytes.Buffer
	require.NoError(t, pdf.Output(&buf))
	media := fmt.Sprintf("/MediaBox [0 0 %.2f %.2f]",
		indexTrimWidth+indexCropOffset, indexTrimHeight)
	require.Contains(t, buf.String(), media)
	cropped := strings.Replace(buf.String(), media, media+fmt.Sprintf(
		" /CropBox [%.2f 0 %.2f %.2f]",
		indexCropOffset, indexTrimWidth+indexCropOffset, indexTrimHeight,
	), 1)

	path := filepath.Join(t.TempDir(), "cropped-index.pdf")
	require.NoError(t, os.WriteFile(path, []byte(cropped), 0o600))
	return path
}

// padToWidth appends filler words until s nearly fills width, so each
// column's lines end flush against the narrow gutter.
func padToWidth(pdf *fpdf.Fpdf, s string, width float64) string {
	for pdf.GetStringWidth(s+" more") < width {
		s += " more"
	}
	return s
}

// TestGoPDFConverter_CroppedNarrowGutterIndex: an index whose columns sit a
// narrow 12pt apart on a cropped page reads column by column.
func TestGoPDFConverter_CroppedNarrowGutterIndex(t *testing.T) {
	epubPath := convertToEPUB(t, makeCroppedIndexPDF(t))
	blocks := extractBlocks(t, string(readZipEntry(t, epubPath)))

	require.Equal(t, "Index", blocks[0].text)
	for _, b := range blocks {
		require.False(t,
			strings.Contains(b.text, "left entry") &&
				strings.Contains(b.text, "right entry"),
			"columns merged: %q", b.text)
	}
	joined := strings.Join(blockTexts(blocks, "p"), "\n")
	require.NotContains(t, joined, "hidden slug")
	require.Less(t,
		strings.Index(joined, "left entry d term, 29"),
		strings.Index(joined, "right entry a term, 0"),
		"left column must be read before the right column")
}

func gutterTestChar(text string, left, right float64) pdfChar {
	return pdfChar{
		text: text, left: left, top: 10, right: right, bottom: 0,
		font: "", stream: noStreamPos,
	}
}

func TestSplitAtGutter(t *testing.T) {
	t.Parallel()

	a, b := gutterTestChar("a", 0, 5), gutterTestChar("b", 20, 25)
	left, right, ok := splitAtGutter([]pdfChar{a, b}, 12)
	require.True(t, ok)
	require.Equal(t, []pdfChar{a}, left)
	require.Equal(t, []pdfChar{b}, right)

	// A glyph across the gutter keeps the line whole.
	_, _, ok = splitAtGutter([]pdfChar{a, gutterTestChar("x", 10, 14), b}, 12)
	require.False(t, ok)
	// One-sided lines have nothing to split.
	_, _, ok = splitAtGutter([]pdfChar{a}, 12)
	require.False(t, ok)
	_, _, ok = splitAtGutter([]pdfChar{b}, 12)
	require.False(t, ok)
}

// TestAssignColumns_HeaderAboveBothColumnsReadsFirst: a recto page's running
// header sits right of the gutter but above both columns, so it reads first.
func TestAssignColumns_HeaderAboveBothColumnsReadsFirst(t *testing.T) {
	t.Parallel()

	header := lineAt("INDEX 217", 330, 640)
	left := lineAt("Daly, Herman, ix, 106", 54, 600)
	right := lineAt("Nasar, Sylvia, 127", 222, 600)
	gotLeft, gotRight := assignColumns([]pdfLine{left, right, header}, 210, 222, true)
	require.Equal(t, "INDEX 217", gotLeft[0].text)
	require.Len(t, gotRight, 1)
}
