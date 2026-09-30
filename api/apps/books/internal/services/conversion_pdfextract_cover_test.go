//nolint:testpackage // testing unexported service helpers
package services

import (
	"image/color"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeCoverAndPartPagePDF: a full-page cover image, then a Part divider page
// with only a few words of text, then a body page.
func makeCoverAndPartPagePDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureImage(t, pdf, "fixture-cover.png", 0, 0, fixturePageWidth,
		fixturePageHeight, color.RGBA{R: 200, G: 90, B: 40, A: 255})

	pdf.AddPage()
	fixtureText(pdf, 300, 300, fixtureBodySize, "PART ONE")
	fixtureText(pdf, 250, 330, fixtureHeadingSize, "System Structure and Behavior")

	pdf.AddPage()
	fixtureText(pdf, fixtureLeftX, 80, fixtureBodySize, singleColParaA)
	fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, singleColParaB)
	return savePDF(t, pdf, "cover-part.pdf")
}

func TestGoPDFConverter_CoverDeclaredAndPartPageKeptAsText(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeCoverAndPartPagePDF(t))
	requireValidKEPUB(t, epubPath)

	opf := string(readZipEntryNamed(t, epubPath, "OEBPS/content.opf"))
	coverItem := regexp.MustCompile(`<item [^>]*properties="cover-image"[^>]*/>`).FindString(opf)
	require.NotEmpty(t, coverItem, opf)
	require.Contains(t, opf, `<meta name="cover" content="`)
	require.Regexp(t, `<spine>\s*<itemref idref="cover"/>`, opf)

	cover := string(readZipEntryNamed(t, epubPath, "OEBPS/cover.xhtml"))
	require.Contains(t, cover, "<img")

	body := string(readZipEntry(t, epubPath))
	require.NotContains(t, body, "<img", "the cover must not repeat in the body")
	var texts []string
	for _, b := range extractBlocks(t, body) {
		texts = append(texts, b.text)
	}
	require.Contains(t, texts, "System Structure and Behavior")
	require.Contains(t, texts, "PART ONE")
}
