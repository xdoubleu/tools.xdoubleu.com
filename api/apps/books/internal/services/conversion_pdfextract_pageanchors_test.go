//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	fixtureText(pdf, fixtureLeftX, 80, fixtureHeadingSize, pagedHeading)
	fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, pagedParaOne)
	pdf.AddPage()
	fixtureText(pdf, fixtureLeftX, 80, fixtureBodySize, pagedParaTwo)
	fixtureText(pdf, fixtureLeftX, 80+fixtureBreakDY, fixtureBodySize, pagedParaTwoB)
	pdf.AddPage()
	fixtureText(pdf, fixtureLeftX, 80, fixtureBodySize, pagedParaThree)
	pdf.AddPage()
	return savePDF(t, pdf, "paged.pdf")
}

var pageAnchorRe = regexp.MustCompile(
	`<span epub:type="pagebreak" id="pdfpage-(\d+)" role="doc-pagebreak"`,
)

func anchoredPages(doc string) []string {
	var pages []string
	for _, m := range pageAnchorRe.FindAllStringSubmatch(doc, -1) {
		pages = append(pages, m[1])
	}
	return pages
}

func TestWriteBlocksHTML_PageAnchors(t *testing.T) {
	//nolint:exhaustruct // only html, tag and page are rendered
	blocks := []htmlBlock{
		{html: "<p>A</p>", tag: "p", page: 1},
		{html: "<p>B</p>", tag: "p"},
		{html: "<li>C</li>", tag: listTag, page: 2},
		{html: `<img src="x.png" alt=""/>`, tag: imgTag, page: 3},
		{html: `<h2 class="toc">T</h2>`, tag: "h2", page: 4},
	}
	var b strings.Builder
	writeBlocksHTML(&b, blocks)

	anchor := func(n string) string {
		return `<span epub:type="pagebreak" id="pdfpage-` + n +
			`" role="doc-pagebreak"></span>`
	}
	got := b.String()
	assert.Contains(t, got, "<p>"+anchor("1")+"A</p>")
	assert.Contains(t, got, "\n<p>B</p>")
	assert.Contains(t, got, "<li>"+anchor("2")+"C</li>")
	assert.Contains(t, got, anchor("3")+`<img src="x.png" alt=""/>`)
	assert.Contains(t, got, `<h2 class="toc">`+anchor("4")+"T</h2>")
}

func TestCaptionFigures_KeepsPage(t *testing.T) {
	//nolint:exhaustruct // only the fields captionFigures reads
	blocks := []htmlBlock{
		{tag: imgTag, src: "f.png", page: 7},
		{tag: "p", text: "Figure 1 A caption", isText: true},
	}
	captionFigures(blocks)
	assert.Contains(t, blocks[0].html, `alt="Figure 1 A caption"`)
	assert.Equal(t, 7, blocks[0].page)
}

func TestJoinPageContinuations_AnchorsThePageAtTheJoin(t *testing.T) {
	half := func(text string) htmlBlock {
		b := flowBlock(text)
		b.inline = text
		return b
	}
	figure := imageBlock("f.png", "Figure")
	pages := [][]htmlBlock{
		{half("The earth")},
		{half("is a system. So is the sun")},
		{figure, half("and the moon.")},
	}
	joined, inline := joinPageContinuations(pages, 10)

	assert.Equal(t, map[int]bool{1: true}, inline,
		"a page opening with a figure keeps its anchor on the figure")
	assert.Equal(
		t,
		"The earth "+pageAnchorHTML(2)+"is a system. So is the sun and the moon.",
		joined[0][0].inline,
	)
}

func TestPageListEntries_OnlyPageBreaks(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body>"+
		`<p><span id="pdfpage-4">Not a page break.</span></p>`+
		"</body></html>", nil)
	zr := convertToEPUBZip(
		t, inPath,
		ArticleMeta{Title: "Book", Authors: nil, Identifier: "", CoverImage: ""},
	)
	assert.NotContains(t, zipEntryContent(t, zr, "OEBPS/nav.xhtml"), "page-list")
}

func TestGoHTMLConverter_PageListNav(t *testing.T) {
	anchor := func(n string) string {
		return `<span epub:type="pagebreak" id="pdfpage-` + n +
			`" role="doc-pagebreak"></span>`
	}
	inPath := writeArticleFixture(t, "<html><body>"+
		"<p>"+anchor("1")+"Front matter.</p>"+
		`<h1 class="toc">`+anchor("2")+`Part One</h1><p>Part intro.</p>`+
		"<p>"+anchor("3")+"More.</p>"+
		"</body></html>", nil)
	zr := convertToEPUBZip(
		t, inPath,
		ArticleMeta{Title: "Book", Authors: nil, Identifier: "", CoverImage: ""},
	)

	nav := zipEntryContent(t, zr, "OEBPS/nav.xhtml")
	assert.Contains(t, nav, `<nav epub:type="page-list" hidden="hidden">`)
	assert.Contains(t, nav, `<li><a href="index.xhtml#pdfpage-1">1</a></li>`)
	assert.Contains(t, nav, `<li><a href="index-1.xhtml#pdfpage-2">2</a></li>`)
	assert.Contains(t, nav, `<li><a href="index-1.xhtml#pdfpage-3">3</a></li>`)
	assert.Contains(t, nav, `<a href="index-1.xhtml#heading-0">Part One</a>`,
		"the anchor adds no text to the TOC title")

	for _, name := range []string{"OEBPS/index.xhtml", "OEBPS/index-1.xhtml"} {
		doc := zipEntryContent(t, zr, name)
		assert.Contains(t, doc, `xmlns:epub="http://www.idpf.org/2007/ops"`, name)
	}
	assert.Equal(t, []string{"2", "3"},
		anchoredPages(zipEntryContent(t, zr, "OEBPS/index-1.xhtml")))
}

func TestGoHTMLConverter_NoPageListWithoutAnchors(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body><p>Plain.</p></body></html>", nil)
	zr := convertToEPUBZip(
		t, inPath,
		ArticleMeta{Title: "Book", Authors: nil, Identifier: "", CoverImage: ""},
	)
	assert.NotContains(t, zipEntryContent(t, zr, "OEBPS/nav.xhtml"), "page-list")
}

// pagedKEPUB converts makePagedPDF and runs the result through kepubify.
func pagedKEPUB(t *testing.T) (string, *zip.Reader) {
	t.Helper()
	epubPath := convertToEPUB(t, makePagedPDF(t))
	data, err := os.ReadFile(epubPath)
	require.NoError(t, err)
	kepub, err := newKepubifyConverter().Convert(context.Background(), data)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(kepub), int64(len(kepub)))
	require.NoError(t, err)
	return epubPath, zr
}

func TestGoPDFConverter_PageAnchorsSurviveKepubify(t *testing.T) {
	t.Parallel()
	epubPath, kepub := pagedKEPUB(t)

	epubDocs := string(readZipEntry(t, epubPath))
	assert.Equal(t, []string{"1", "2", "3"}, anchoredPages(epubDocs),
		"one anchor per page with text, in order; none for the blank page")

	nav := string(readZipEntryNamed(t, epubPath, "OEBPS/nav.xhtml"))
	assert.Contains(t, nav, `<nav epub:type="page-list" hidden="hidden">`)
	assert.Equal(t, 3, strings.Count(nav, "#pdfpage-"))

	var kepubDocs string
	for i := 0; ; i++ {
		f, err := kepub.Open("OEBPS/" + contentDocName(i))
		if err != nil {
			break
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(f)
		require.NoError(t, err)
		_ = f.Close()
		kepubDocs += buf.String()
	}
	assert.Equal(t, []string{"1", "2", "3"}, anchoredPages(kepubDocs),
		"kepubify keeps the anchors")
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
