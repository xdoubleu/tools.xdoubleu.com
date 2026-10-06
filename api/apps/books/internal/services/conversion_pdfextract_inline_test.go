//nolint:testpackage // testing unexported service helpers
package services

import (
	"regexp"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/require"
)

// styledRun draws text in a style at x, returning the x after it.
func styledRun(pdf *fpdf.Fpdf, x, y float64, style, s string) float64 {
	pdf.SetFont("Times", style, fixtureBodySize)
	pdf.Text(x, y, s)
	return x + pdf.GetStringWidth(s)
}

// makeInlineStylesPDF: a paragraph with an italic word, a bold term, and a
// superscript note number, then a paragraph set entirely in bold.
func makeInlineStylesPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	x := styledRun(pdf, fixtureLeftX, 80, "", "The word ")
	x = styledRun(pdf, x, 80, "I", "function")
	x = styledRun(pdf, x, 80, "", " is used for a ")
	x = styledRun(pdf, x, 80, "B", "system")
	x = styledRun(pdf, x, 80, "", ", said Anderson")
	pdf.SetFont("Times", "", 6)
	pdf.Text(x+0.3, 76, "1")
	styledRun(pdf, fixtureLeftX, 80+fixtureBreakDY, "B",
		"A margin note set wholly in bold type.")
	return savePDF(t, pdf, "inline-styles.pdf")
}

func TestGoPDFConverter_InlineStyles(t *testing.T) {
	t.Parallel()
	doc := string(readZipEntry(t, convertToEPUB(t, makeInlineStylesPDF(t))))
	doc = pageBreakRe.ReplaceAllString(doc, "")
	paras := regexp.MustCompile(`<p>(.*?)</p>`).FindAllStringSubmatch(doc, -1)
	require.Len(t, paras, 2, doc)
	require.Equal(t,
		"The word <em>function</em> is used for a <strong>system</strong>, "+
			"said Anderson<sup>1</sup>",
		paras[0][1])
	require.Equal(t, "A margin note set wholly in bold type.", paras[1][1])
}

func TestParagraphHTML_MergesRunsAcrossLines(t *testing.T) {
	t.Parallel()

	line := func(text string, bottom float64) pdfLine {
		var chars []pdfChar
		x := 10.0
		for _, r := range text {
			if r != ' ' {
				chars = append(chars, pdfChar{
					text: string(r), left: x, top: bottom + 7, right: x + 3.5,
					bottom: bottom, font: "Verlag-Italic", stream: noStreamPos,
				})
			}
			x += 4
		}
		return buildLine(chars)
	}
	got := paragraphHTML(
		[]pdfLine{line("the interconnec-", 100), line("tions hold", 90)},
	)
	require.Equal(t, "<em>the interconnections hold</em>", got)
}
