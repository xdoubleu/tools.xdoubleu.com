//nolint:testpackage // testing the unexported text model
package services

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
)

func physicalText(t *testing.T, printPages bool) *bookText {
	t.Helper()
	bt, err := buildBookText(context.Background(),
		openZip(t, mocks.PhysicalEPUB("Physical", printPages)))
	require.NoError(t, err)
	return bt
}

func pos(href string, offset int) models.ReadingPosition {
	return models.ReadingPosition{Href: href, Offset: offset, Page: 0}
}

func TestBuildBookText_SpineStartAndChapters(t *testing.T) {
	bt := physicalText(t, false)

	hrefs := make([]string, len(bt.docs))
	for i, d := range bt.docs {
		hrefs[i] = d.href
	}
	assert.Equal(t, []string{
		mocks.PhysicalFrontHref, mocks.PhysicalCh1Href,
		mocks.PhysicalCh2Href, mocks.PhysicalCh3Href,
	}, hrefs, "the nav document isn't in the spine")
	assert.Equal(t, "Title page. Copyright.", str(bt.docs[0].units))
	assert.Equal(t, bt.cum[1], bt.start, "bodymatter landmark")
	assert.False(t, bt.hasPrintPages())

	require.Len(t, bt.chapters, 3)
	assert.Equal(t, "Chapter Two", bt.chapters[1].title)
	assert.Equal(t, pos(mocks.PhysicalCh2Href, len("Epigraph.")),
		bt.position(bt.chapters[1].at), "#c2 lands after the epigraph")
	assert.Equal(t, pos(mocks.PhysicalCh3Href, 0), bt.position(bt.chapters[2].at))
}

func TestBookText_PrintPagesMapExactly(t *testing.T) {
	bt := physicalText(t, true)
	require.True(t, bt.hasPrintPages())
	half := len(mocks.PhysicalSentence(1, 1)) * mocks.PhysicalSentences / 2
	c := models.PhysicalCopy{PageCount: 0, TextStartPage: 0, Anchors: nil, Syncable: true}

	got, _, ok := bt.positionOfPage(c, 2)
	require.True(t, ok)
	assert.Equal(t, pos(mocks.PhysicalCh1Href, half), got)
	got, _, _ = bt.positionOfPage(c, 3)
	assert.Equal(t, pos(mocks.PhysicalCh2Href, 0), got)
	got, _, _ = bt.positionOfPage(c, 9)
	assert.Equal(t, pos(mocks.PhysicalCh3Href, half), got, "past the end: the last page")

	assert.Equal(t, 1, bt.pageOf(c, pos(mocks.PhysicalFrontHref, 3)), "roman front matter")
	assert.Equal(t, 1, bt.pageOf(c, pos(mocks.PhysicalCh1Href, half-1)))
	assert.Equal(t, 2, bt.pageOf(c, pos(mocks.PhysicalCh1Href, half)))
	assert.Equal(t, 6, bt.pageOf(c, pos(mocks.PhysicalCh3Href, 1<<20)))
	assert.Equal(t, 0, bt.pageOf(c, pos("OEBPS/missing.xhtml", 0)))
	assert.Equal(t, 0, bt.pageOf(c, models.ReadingPosition{Href: "", Offset: 0, Page: 4}))
}

func TestBookText_InterpolatesBetweenTextStartAndEnd(t *testing.T) {
	bt := physicalText(t, false)
	c := models.PhysicalCopy{PageCount: 99, TextStartPage: 1, Anchors: nil, Syncable: true}

	got, g, ok := bt.positionOfPage(c, 1)
	require.True(t, ok)
	assert.Equal(t, pos(mocks.PhysicalCh1Href, 0), got)
	assert.Equal(t, bt.cum[1], g)
	for page := 1; page <= c.PageCount; page++ {
		p, _, _ := bt.positionOfPage(c, page)
		assert.Equal(t, page, bt.pageOf(c, p), "page %d round-trips", page)
	}
	assert.Equal(t, 1, bt.pageOf(c, pos(mocks.PhysicalFrontHref, 0)), "before the text")
	assert.Equal(t, 99, bt.pageOf(c, pos(mocks.PhysicalCh3Href, 1<<20)))
	got, _, _ = bt.positionOfPage(c, 500)
	assert.Equal(t, mocks.PhysicalCh3Href, got.Href)

	c.PageCount = 0
	_, _, ok = bt.positionOfPage(c, 3)
	assert.False(t, ok, "no page count, nothing to interpolate over")
	assert.Equal(t, 0, bt.pageOf(c, pos(mocks.PhysicalCh1Href, 0)))
}

func TestBookText_AnchorsPinPages(t *testing.T) {
	bt := physicalText(t, false)
	chapterTwo := bt.position(bt.chapters[1].at)
	c := models.PhysicalCopy{
		PageCount: 60, TextStartPage: 5, Syncable: true,
		Anchors: []models.PhysicalAnchor{
			{Page: 40, Href: mocks.PhysicalCh1Href, Offset: 10}, // before page 30: dropped
			{Page: 30, Href: chapterTwo.Href, Offset: chapterTwo.Offset},
			{Page: 50, Href: "OEBPS/missing.xhtml", Offset: 0},
		},
	}
	got, _, _ := bt.positionOfPage(c, 30)
	assert.Equal(t, chapterTwo, got)
	assert.Equal(t, 30, bt.pageOf(c, chapterTwo))
	assert.Equal(t, 5, bt.pageOf(c, pos(mocks.PhysicalCh1Href, 0)))
	assert.Greater(t, bt.pageOf(c, pos(mocks.PhysicalCh3Href, 0)), 30)

	// An anchor on or before the text start replaces it.
	c.Anchors = []models.PhysicalAnchor{{Page: 3, Href: mocks.PhysicalCh1Href, Offset: 50}}
	assert.Equal(t, 3, bt.pageOf(c, pos(mocks.PhysicalCh1Href, 50)))
	assert.Equal(t, 3, bt.pageOf(c, pos(mocks.PhysicalCh1Href, 0)))
	// So does a later page pinned before the body matter starts.
	c.Anchors = []models.PhysicalAnchor{
		{Page: 7, Href: mocks.PhysicalFrontHref, Offset: 0},
	}
	assert.Equal(t, 7, bt.pageOf(c, pos(mocks.PhysicalFrontHref, 0)))
	// One past the last page replaces the end.
	c.Anchors = []models.PhysicalAnchor{{Page: 61, Href: mocks.PhysicalCh2Href, Offset: 0}}
	assert.Equal(t, 60, bt.pageOf(c, pos(mocks.PhysicalCh3Href, 0)))
}

func TestBookText_ExcerptAndPercent(t *testing.T) {
	bt := physicalText(t, false)
	assert.Equal(t, "Chapter 1 sentence 01 is right here.",
		bt.excerpt(pos(mocks.PhysicalCh1Href, 0)))
	assert.Equal(t, "Title page. Copyright.", bt.excerpt(pos(mocks.PhysicalFrontHref, 0)),
		"too short to be a sentence end")
	assert.Empty(t, bt.excerpt(pos("OEBPS/missing.xhtml", 0)))

	long := strings.Repeat("word ", 100)
	lt := &bookText{
		docs:  []textDoc{{href: "a", units: []uint16(nil), gaps: nil}},
		cum:   []int{0, len(long)},
		start: 0, printPages: nil, chapters: nil,
	}
	for _, r := range long {
		lt.docs[0].units = append(lt.docs[0].units, uint16(r))
	}
	ex := lt.excerpt(pos("a", 0))
	assert.True(t, strings.HasSuffix(ex, "word…"), ex)
	assert.LessOrEqual(t, len([]rune(ex)), maxExcerptRunes+1)

	assert.Equal(t, 0, bt.percentAt(0))
	assert.Equal(t, 50, bt.percentAt(bt.total()/2))
	assert.Equal(t, 100, bt.percentAt(bt.total()))
	empty := &bookText{docs: nil, cum: []int{0}, start: 0, printPages: nil, chapters: nil}
	assert.Equal(t, 0, empty.percentAt(0))
}

func TestPrintPageMarker(t *testing.T) {
	el := func(attrs ...xml.Attr) xml.StartElement {
		return xml.StartElement{Name: xml.Name{Space: "", Local: "span"}, Attr: attrs}
	}
	a := func(space, local, value string) xml.Attr {
		return xml.Attr{Name: xml.Name{Space: space, Local: local}, Value: value}
	}
	cases := []struct {
		name  string
		e     xml.StartElement
		label string
		ok    bool
	}{
		{"title", el(a("epub", "type", "pagebreak"), a("", "title", "12")), "12", true},
		{
			"aria",
			el(a("", "role", "doc-pagebreak"), a("", "aria-label", "Page 7")),
			"7",
			true,
		},
		{"id digits", el(a("http://www.idpf.org/2007/ops", "type", "pagebreak"),
			a("", "id", "p034")), "034", true},
		{"no label", el(a("epub", "type", "pagebreak"), a("", "id", "page")), "", false},
		{"not a marker", el(a("", "type", "pagebreak"), a("", "title", "3")), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, ok := printPageMarker(tc.e)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.label, label)
		})
	}
}

func TestBuildBookText_BodyMarkersWithoutPageList(t *testing.T) {
	zr := epub2(t, map[string]string{
		"OEBPS/a.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"` +
			` xmlns:epub="http://www.idpf.org/2007/ops"><body>` +
			`<p>Front.</p><span epub:type="pagebreak" title="1"/><p>One.</p>` +
			`<span role="doc-pagebreak" aria-label="2"/><p>Two.</p></body></html>`,
	}, "")
	bt, err := buildBookText(context.Background(), zr)
	require.NoError(t, err)
	require.Len(t, bt.printPages, 2)
	assert.Equal(t, printPage{label: "1", at: len("Front.")}, bt.printPages[0])
	assert.Equal(t, printPage{label: "2", at: len("Front.One.")}, bt.printPages[1])
}

func TestBuildBookText_NCXAndGuide(t *testing.T) {
	ncx := `<ncx><navMap>` +
		`<navPoint><navLabel><text>Part One</text></navLabel>` +
		`<content src="Text/b.xhtml"/>` +
		`<navPoint><navLabel><text>Nested</text></navLabel>` +
		`<content src="Text/b.xhtml#n"/></navPoint></navPoint>` +
		`</navMap><pageList><pageTarget><navLabel><text>7</text></navLabel>` +
		`<content src="Text/b.xhtml#n"/></pageTarget></pageList></ncx>`
	zr := epub2(t, map[string]string{
		"OEBPS/Text/a.xhtml": `<html><body><p>Cover text.</p></body></html>`,
		"OEBPS/Text/b.xhtml": `<html><body><p>Intro.</p><p id="n">Here.</p></body></html>`,
		"OEBPS/toc.ncx":      ncx,
	}, `<guide><reference type="Text" href="Text/b.xhtml"/></guide>`)
	bt, err := buildBookText(context.Background(), zr)
	require.NoError(t, err)

	assert.Equal(t, bt.cum[1], bt.start, "guide text reference")
	require.Len(t, bt.chapters, 2)
	assert.Equal(t, "Nested", bt.chapters[1].title)
	assert.Equal(t, bt.cum[1]+len("Intro."), bt.chapters[1].at)
	assert.Equal(
		t,
		[]printPage{{label: "7", at: bt.cum[1] + len("Intro.")}},
		bt.printPages,
	)
}

func TestBuildBookText_Broken(t *testing.T) {
	_, err := buildBookText(context.Background(), zipOf(t, nil))
	require.Error(t, err, "no container.xml")

	_, err = buildBookText(context.Background(), epub2(t, nil, ""))
	require.Error(t, err, "empty spine")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = buildBookText(ctx, openZip(t, mocks.PhysicalEPUB("x", false)))
	require.ErrorIs(t, err, context.Canceled)
}

// epub2 builds an EPUB2 whose spine is files' .xhtml entries in name order,
// with toc.ncx as its NCX when present.
func epub2(t *testing.T, files map[string]string, guide string) *zip.Reader {
	t.Helper()
	var manifest, spine strings.Builder
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for i, name := range names {
		rel := strings.TrimPrefix(name, "OEBPS/")
		if strings.HasSuffix(name, ".ncx") {
			manifest.WriteString(`<item id="ncx" href="` + rel +
				`" media-type="application/x-dtbncx+xml"/>`)
			continue
		}
		id := "d" + string(rune('a'+i))
		manifest.WriteString(`<item id="` + id + `" href="` + rel +
			`" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="` + id + `"/>`)
	}
	all := map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile` +
			` full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>` +
			`</rootfiles></container>`,
		"OEBPS/content.opf": `<package><manifest>` + manifest.String() +
			`</manifest><spine toc="ncx">` + spine.String() + `</spine>` + guide +
			`</package>`,
	}
	for k, v := range files {
		all[k] = v
	}
	return zipOf(t, all)
}

// ncxOnlyBy builds an EPUB whose NCX is found only by the spine's toc
// attribute or only by its media type, behind a nav document with no links.
func ncxOnlyBy(t *testing.T, byID bool) *zip.Reader {
	t.Helper()
	ncxItem := `<item id="toc" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
	spine := `<spine>`
	if byID {
		ncxItem = `<item id="toc" href="toc.ncx" media-type="text/xml"/>`
		spine = `<spine toc="toc">`
	}
	return zipOf(t, map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile` +
			` full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>` +
			`</rootfiles></container>`,
		"OEBPS/content.opf": `<package><manifest>` +
			`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml"` +
			` properties="nav"/>` + ncxItem +
			`<item id="a" href="a.xhtml" media-type="application/xhtml+xml"/>` +
			`</manifest>` + spine + `<itemref idref="a"/></spine></package>`,
		"OEBPS/nav.xhtml": `<html><body><p>No navs here.</p></body></html>`,
		"OEBPS/a.xhtml":   `<html><body><p>Text.</p></body></html>`,
		"OEBPS/toc.ncx": `<ncx><navMap><navPoint><navLabel><text>Only</text>` +
			`</navLabel><content src="a.xhtml"/></navPoint></navMap></ncx>`,
	})
}

func TestBuildBookText_EmptyNavFallsBackToNCX(t *testing.T) {
	for _, byID := range []bool{true, false} {
		bt, err := buildBookText(context.Background(), ncxOnlyBy(t, byID))
		require.NoError(t, err)
		require.Len(t, bt.chapters, 1, "by id: %v", byID)
		assert.Equal(t, "Only", bt.chapters[0].title)
	}
}
