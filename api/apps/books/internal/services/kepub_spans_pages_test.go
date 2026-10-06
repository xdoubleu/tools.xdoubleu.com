//nolint:testpackage // testing unexported span-map helpers
package services

import (
	"archive/zip"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func pagePos(page int) models.ReadingPosition {
	return models.ReadingPosition{Href: "", Offset: 0, Page: page}
}

func span(id, text string) string {
	return `<span class="koboSpan" id="` + id + `">` + text + `</span>`
}

func pageBreak(n string) string {
	return `<span epub:type="pagebreak" id="pdfpage-` + n +
		`" role="doc-pagebreak"></span>`
}

const (
	pagedCover = "OEBPS/cover.xhtml"
	pagedDoc0  = "OEBPS/index.xhtml"
	pagedDoc1  = "OEBPS/index-1.xhtml"
)

// pagedKEPUBZip is a PDF-sourced KEPUB: a cover, then pages 2, 3 and 5
// anchored (1 is the cover, 4 blank), and page 9's anchor after every span.
func pagedKEPUBZip(t *testing.T) *zip.Reader {
	t.Helper()
	return openZip(t, pagedKEPUBBytes(t))
}

func pagedKEPUBBytes(t *testing.T) []byte {
	t.Helper()
	opf := `<package xmlns="http://www.idpf.org/2007/opf"><manifest>
<item href="cover.xhtml" media-type="application/xhtml+xml"/>
<item href="index.xhtml" media-type="application/xhtml+xml"/>
<item href="index-1.xhtml" media-type="application/xhtml+xml"/>
</manifest></package>`
	body := func(s string) string {
		return `<html xmlns="http://www.w3.org/1999/xhtml"><body>` + s +
			"</body></html>"
	}
	return zipBytesOf(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf":      opf,
		pagedCover:               body(`<p>` + span("kobo.1.1", "Cover") + `</p>`),
		pagedDoc0: body(`<p>` + pageBreak("2") + span("kobo.1.1", "Alpha.") +
			`</p><p>` + span("kobo.2.1", "Beta.") + `</p>`),
		pagedDoc1: body(`<p>` + span("kobo.1.1", "Gamma.") + ` ` + pageBreak("3") +
			span("kobo.1.2", "Delta.") + `</p><h2>` + pageBreak("5") +
			span("kobo.2.1", "Eps") + `</h2><p>` + span("kobo.3.1", "Zeta") +
			`</p>` + pageBreak("9")),
	})
}

func pagedMap(t *testing.T) *spanMap {
	t.Helper()
	m, err := buildSpanMap(context.Background(), pagedKEPUBZip(t), nil)
	require.NoError(t, err)
	return m
}

func spanLoc(source, value string) *models.KoboLocation {
	return &models.KoboLocation{Source: source, Type: koboSpanType, Value: value}
}

func TestBuildSpanMap_PDFSourcedRecordsPageAnchors(t *testing.T) {
	m := pagedMap(t)
	require.True(t, m.pdf)
	require.Len(t, m.docs, 3)
	assert.Equal(t, []pageAnchor{
		{page: 2, doc: 1, start: 0},
		{page: 3, doc: 2, start: int32(len("Gamma. "))},
		{page: 5, doc: 2, start: int32(len("Gamma. Delta."))},
		{page: 9, doc: 2, start: int32(len("Gamma. Delta.EpsZeta"))},
	}, m.pages)
	assert.Equal(t, int32(len("Gamma. ")), m.docs[2].spans[1].start,
		"offsets are the KEPUB's own text")
}

func TestSpanMap_PageToSpan(t *testing.T) {
	m := pagedMap(t)
	for _, tc := range []struct {
		page int
		want *models.KoboLocation
	}{
		{1, spanLoc(pagedDoc0, "kobo.1.1")}, // before every anchor: the first
		{2, spanLoc(pagedDoc0, "kobo.1.1")},
		{3, spanLoc(pagedDoc1, "kobo.1.2")},
		{4, spanLoc(pagedDoc1, "kobo.1.2")}, // no anchor: the page before
		{5, spanLoc(pagedDoc1, "kobo.2.1")},
		{9, nil}, // no span at or after the anchor
		{40, nil},
	} {
		assert.Equal(t, tc.want, m.location(pagePos(tc.page)), "page %d", tc.page)
	}
}

func TestSpanMap_SpanToPage(t *testing.T) {
	m := pagedMap(t)
	for _, tc := range []struct {
		source, value string
		page          int
	}{
		{pagedCover, "kobo.1.1", 1},
		{pagedDoc0, "kobo.1.1", 2},
		{pagedDoc0, "kobo.2.1", 2},
		{pagedDoc1, "kobo.1.1", 2},
		{pagedDoc1, "kobo.1.2", 3},
		{pagedDoc1, "kobo.3.1", 5},
	} {
		pos := m.position(*spanLoc(tc.source, tc.value))
		require.NotNil(t, pos, tc.value)
		assert.Equal(t, pagePos(tc.page), *pos, "%s %s", tc.source, tc.value)
	}
	assert.Nil(t, m.position(*spanLoc(pagedDoc1, "kobo.7.7")))
}

func TestSpanMap_HrefOffsetOnPDFSourcedKEPUB(t *testing.T) {
	m := pagedMap(t)
	assert.Equal(t, spanLoc(pagedDoc1, "kobo.1.2"), m.location(
		models.ReadingPosition{Href: pagedDoc1, Offset: len("Gamma. De"), Page: 0},
	), "KEPUB offsets map to spans as for an EPUB")
}

func TestSpanMap_ReaderPosition(t *testing.T) {
	m := pagedMap(t)
	delta := len("Gamma. ")

	assert.Equal(t, &models.ReadingPosition{Href: pagedDoc1, Offset: delta, Page: 3},
		m.readerPosition(pagePos(3)), "a page gains its anchor")
	assert.Equal(t, &models.ReadingPosition{Href: pagedDoc1, Offset: delta, Page: 4},
		m.readerPosition(pagePos(4)), "the stored page is kept")
	assert.Equal(t,
		&models.ReadingPosition{Href: pagedDoc1, Offset: delta + 2, Page: 3},
		m.readerPosition(models.ReadingPosition{
			Href: "index-1.xhtml", Offset: delta + 2, Page: 0,
		}), "an offset gains the page whose anchor precedes it")

	assert.Nil(t, m.readerPosition(models.ReadingPosition{
		Href: "OEBPS/missing.xhtml", Offset: 0, Page: 0,
	}))
	assert.Nil(t, (&spanMap{docs: m.docs, pages: nil, pdf: true}).
		readerPosition(pagePos(3)), "no anchors")
}

func TestSpanMap_EPUBSourcedHasNoPages(t *testing.T) {
	orig, kepub := chapterZips(t)
	m, err := buildSpanMap(context.Background(), kepub, orig)
	require.NoError(t, err)
	assert.False(t, m.pdf)
	assert.Empty(t, m.pages)
	assert.Nil(t, m.location(pagePos(1)))
	assert.Nil(t, m.readerPosition(pagePos(1)))
}

func TestParsePageAnchorID(t *testing.T) {
	for id, want := range map[string]int{
		"pdfpage-1": 1, "pdfpage-120": 120, "pdfpage-0": 0, "pdfpage-": 0,
		"pdfpage-x": 0, "heading-3": 0, "pdfpage--2": 0,
	} {
		assert.Equal(t, want, parsePageAnchorID(id), id)
	}
}
