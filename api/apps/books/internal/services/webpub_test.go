//nolint:testpackage // testing unexported manifest helpers
package services

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
)

func TestBuildWebPubManifest_ChapterEPUB(t *testing.T) {
	data := mocks.ChapterEPUB("Title", "Author")
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	m, err := buildWebPubManifest(zr)
	require.NoError(t, err)

	assert.Equal(t, "Title", m.Metadata.Title)
	assert.Equal(t, []string{"Author"}, m.Metadata.Author)
	require.Len(t, m.ReadingOrder, 3)
	assert.Equal(t, "res/"+mocks.ChapterOneHref, m.ReadingOrder[0].Href)
	assert.Equal(t, "application/xhtml+xml", m.ReadingOrder[0].Type)
	assert.Equal(t, "self", m.Links[0].Rel[0])
	assert.Equal(t, "positions.json", m.Links[1].Href)
	assert.Equal(t, "application/vnd.readium.position-list+json", m.Links[1].Type)
}

func TestBuildWebPubPositions(t *testing.T) {
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata>
<manifest>
<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
<item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`
	zr := zipOf(t, map[string]string{
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      opf,
		"OEBPS/c1.xhtml":         strings.Repeat("x", 2*1024+1),
		"OEBPS/c2.xhtml":         "",
	})
	m, err := buildWebPubManifest(zr)
	require.NoError(t, err)

	p := buildWebPubPositions(zr, m)

	assert.Equal(t, 4, p.Total, "3 for 2049 bytes, at least 1 for an empty section")
	require.Len(t, p.Positions, 4)
	for i, loc := range p.Positions {
		assert.Equal(t, i+1, loc.Locations.Position)
		assert.InDelta(t, float64(i)/4, loc.Locations.TotalProgression, 1e-9)
	}
	assert.Equal(t, "res/OEBPS/c1.xhtml", p.Positions[2].Href)
	assert.Equal(t, "application/xhtml+xml", p.Positions[2].Type)
	assert.InDelta(t, 2.0/3, p.Positions[2].Locations.Progression, 1e-9)
	assert.Equal(t, "res/OEBPS/c2.xhtml", p.Positions[3].Href)
	assert.Zero(t, p.Positions[3].Locations.Progression)
}

const navOPF = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
<item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/>
<item id="img" href="cover.jpg" media-type="image/jpeg" properties="cover-image"/>
</manifest>
<spine><itemref idref="c1"/><itemref idref="c2" linear="no"/></spine>
</package>`

const containerXML = `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
<rootfiles><rootfile full-path="OEBPS/content.opf"
 media-type="application/oebps-package+xml"/></rootfiles>
</container>`

func TestBuildWebPubManifest_NavTOCAndResources(t *testing.T) {
	zr := zipOf(t, map[string]string{
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      navOPF,
		"OEBPS/nav.xhtml": `<html xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
<li><a href="c1.xhtml">One</a><ol><li><a href="c1.xhtml#s">Sub</a></li></ol></li>
<li><a href="c2.xhtml">Two</a></li>
</ol></nav></body></html>`,
	})

	m, err := buildWebPubManifest(zr)
	require.NoError(t, err)

	require.Len(t, m.ReadingOrder, 1, "non-linear items are skipped")
	require.Len(t, m.TOC, 2)
	assert.Equal(t, "One", m.TOC[0].Title)
	assert.Equal(t, "res/OEBPS/c1.xhtml", m.TOC[0].Href)
	require.Len(t, m.TOC[0].Children, 1)
	assert.Equal(t, "res/OEBPS/c1.xhtml#s", m.TOC[0].Children[0].Href)

	var cover bool
	for _, r := range m.Resources {
		if r.Href == "res/OEBPS/cover.jpg" && len(r.Rel) == 1 && r.Rel[0] == "cover" {
			cover = true
		}
	}
	assert.True(t, cover, "cover-image item is a cover resource")
}

func TestBuildWebPubManifest_NCXFallback(t *testing.T) {
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title></metadata>
<manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
<item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest>
<spine toc="ncx"><itemref idref="c1"/></spine></package>`
	zr := zipOf(t, map[string]string{
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      opf,
		"OEBPS/toc.ncx": `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap>
<navPoint><navLabel><text>One</text></navLabel>
<content src="c1.xhtml"/></navPoint></navMap></ncx>`,
	})

	m, err := buildWebPubManifest(zr)
	require.NoError(t, err)
	require.Len(t, m.TOC, 1)
	assert.Equal(t, "One", m.TOC[0].Title)
	assert.Equal(t, "res/OEBPS/c1.xhtml", m.TOC[0].Href)
}

func TestBuildWebPubManifest_NoPackage(t *testing.T) {
	zr := zipOf(t, map[string]string{"x": "y"})
	_, err := buildWebPubManifest(zr)
	require.Error(t, err)
}

func TestResourceType(t *testing.T) {
	assert.Equal(t, "application/xhtml+xml", resourceType("a/b.xhtml"))
	assert.Equal(t, "application/x-dtbncx+xml", resourceType("toc.ncx"))
	assert.Equal(t, "image/jpeg", resourceType("c.jpg"))
	assert.Equal(t, "application/octet-stream", resourceType("c.unknownext"))
}

func TestWebPubResource_RejectsTraversal(t *testing.T) {
	svc := newWebPubService(nil)
	for _, name := range []string{"../etc/passwd", "", "/.."} {
		_, err := svc.Resource(t.Context(), "u", [16]byte{}, name)
		assert.ErrorIs(t, err, ErrWebPubResourceNotFound, name)
	}
}
