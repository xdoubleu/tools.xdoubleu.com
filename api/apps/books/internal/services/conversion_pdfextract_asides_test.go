//nolint:testpackage // testing unexported service helpers
package services

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/require"
)

// Margin-note page geometry from the reference book: body text on a 14.5pt
// leading wraps around a note set in another font on a 13pt leading, so
// note and body baselines drift past each other.
const (
	asideBodyLeft   = 55.0
	asideBodyRight  = 382.0
	asideWrapLeft   = 157.0
	asideNoteLeft   = 43.0
	asideNoteWidth  = 105.0
	asideBodyLead   = 14.5
	asideNoteLead   = 13.0
	asideBodyTop    = 80.0
	asideFullBefore = 3
	asideWrapped    = 7
	asideFullAfter  = 4
)

//nolint:gochecknoglobals // read-only fixture text
var asideNoteLines = []string{
	"Many of the interconnec-",
	"tions in systems operate",
	"through the flow of infor-",
	"mation. Information holds",
	"systems together and plays",
	"a great role in determining",
	"how they operate.",
}

// asideBodyWords is the body paragraph's text, word by word, dealt out
// across full-width and wrapped lines by makeMarginNotePDF.
func asideBodyWords() []string {
	return strings.Fields(strings.Repeat(
		"bodyword stock flow loop delay buffer ", 40,
	))
}

// fillLine takes words until the line would exceed width.
func fillLine(pdf *fpdf.Fpdf, words []string, width float64) (string, []string) {
	line := words[0]
	i := 1
	for i < len(words) && pdf.GetStringWidth(line+" "+words[i]) < width {
		line += " " + words[i]
		i++
	}
	return line, words[i:]
}

func makeMarginNotePDF(t *testing.T) (string, string) {
	t.Helper()
	pdf := newFixturePDF()
	pdf.SetFont("Times", "", 11)

	words := asideBodyWords()
	var body []string
	y := asideBodyTop
	var noteTop float64
	for row := range asideFullBefore + asideWrapped + asideFullAfter {
		left := asideBodyLeft
		if row >= asideFullBefore && row < asideFullBefore+asideWrapped {
			left = asideWrapLeft
			if row == asideFullBefore {
				noteTop = y + 3.5
			}
		}
		var line string
		line, words = fillLine(pdf, words, asideBodyRight-left)
		pdf.Text(left, y, line)
		body = append(body, line)
		y += asideBodyLead
	}

	pdf.SetFont("Helvetica", "B", 8)
	for i, line := range asideNoteLines {
		pdf.Text(asideNoteLeft, noteTop+float64(i)*asideNoteLead, line)
	}
	return savePDF(t, pdf, "margin-note.pdf"), strings.Join(body, " ")
}

// TestGoPDFConverter_MarginNoteKeptApartFromBody: a margin note never splices
// into the body paragraph it sits beside; it comes out whole, as its own
// blockquote.
func TestGoPDFConverter_MarginNoteKeptApartFromBody(t *testing.T) {
	pdfPath, bodyText := makeMarginNotePDF(t)
	doc := string(readZipEntry(t, convertToEPUB(t, pdfPath)))

	quoteRe := regexp.MustCompile(`(?s)<blockquote>(.*?)</blockquote>`)
	quotes := quoteRe.FindAllStringSubmatch(doc, -1)
	require.Len(t, quotes, 1, "want one note blockquote in %s", doc)
	noteText := strings.Join(
		strings.Fields(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(quotes[0][1], " ")),
		" ",
	)
	require.Equal(t,
		"Many of the interconnections in systems operate through the flow of "+
			"information. Information holds systems together and plays a great "+
			"role in determining how they operate.",
		noteText,
	)

	outside := quoteRe.ReplaceAllString(doc, "")
	bodyParas := strings.Join(blockTexts(extractBlocks(t, outside), "p"), " ")
	require.Equal(t, bodyText, bodyParas)
}
