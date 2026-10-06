//nolint:testpackage // testing unexported span-map helpers
package services

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipOf builds an archive of name → content, stored uncompressed.
func zipOf(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	return openZip(t, zipBytesOf(t, files))
}

func zipBytesOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		//nolint:exhaustruct // a stored entry needs only these
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

const testContainer = `<?xml version="1.0"?>` +
	`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"` +
	` version="1.0"><rootfiles>` +
	`<rootfile full-path="other.xml" media-type="application/xml"/>` +
	`<rootfile full-path="OEBPS/content.opf"` +
	` media-type="application/oebps-package+xml"/></rootfiles></container>`

func TestEPUBContentDocs_KepubifyRules(t *testing.T) {
	opf := `<package xmlns="http://www.idpf.org/2007/opf"><manifest>
<item href="Text/ch1.xhtml" media-type="application/xhtml+xml"/>
<item href="Text/page.txt" media-type="text/html"/>
<item href="Text/ch2.HTM" media-type="application/xml"/>
<item href="Text/ch3.html" media-type="text/xml"/>
<item href="Text/ch4.xhtml" media-type="application/octet-stream"/>
<item href="Text/ch%205.xhtml#start" media-type="application/xhtml+xml"/>
<item href="Text/bad%zz.xhtml" media-type="application/xhtml+xml"/>
<item href="Styles/style.css" media-type="text/css"/>
<item href="Images/a.png" media-type="image/png"/>
<item href="toc.ncx" media-type="application/x-dtbncx+xml"/>
</manifest></package>`
	docs, err := epubContentDocs(zipOf(t, map[string]string{
		"META-INF/container.xml": testContainer,
		"OEBPS/content.opf":      opf,
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"OEBPS/Text/ch1.xhtml",
		"OEBPS/Text/page.txt",
		"OEBPS/Text/ch2.HTM",
		"OEBPS/Text/ch3.html",
		"OEBPS/Text/ch4.xhtml",
		"OEBPS/Text/ch 5.xhtml",
		"OEBPS/Text/bad%zz.xhtml",
	}, docs)
}

func TestEPUBContentDocs_NoPackageDocument(t *testing.T) {
	_, err := epubContentDocs(zipOf(t, map[string]string{
		"META-INF/container.xml": `<container><rootfiles>` +
			`<rootfile full-path="x.opf" media-type="application/xml"/>` +
			`</rootfiles></container>`,
	}))
	require.Error(t, err)

	_, err = epubContentDocs(zipOf(t, map[string]string{
		"META-INF/container.xml": testContainer,
	}))
	require.Error(t, err, "the named OPF is missing")
}

func TestZipDocText_SizeCap(t *testing.T) {
	doc := func(size int) string {
		head, tail := "<html><body>", "</body></html>"
		return head + strings.Repeat("a", size-len(head)-len(tail)) + tail
	}
	zr := zipOf(t, map[string]string{
		"max.xhtml":  doc(maxContentDocBytes),
		"over.xhtml": doc(maxContentDocBytes + 1),
	})

	dt, err := zipDocText(zr, "max.xhtml")
	require.NoError(t, err)
	assert.Len(t, dt.units, maxContentDocBytes-len("<html><body></body></html>"))

	_, err = zipDocText(zr, "over.xhtml")
	require.Error(t, err)

	_, err = zipDocText(zr, "missing.xhtml")
	require.Error(t, err)
}

func TestParseBodyText_NestedKoboSpansKeepTheOuter(t *testing.T) {
	dt, err := parseBodyText(strings.NewReader(`<html><body><p>` +
		`<span class="koboSpan" id="kobo.1.1">a` +
		`<span class="koboSpan" id="kobo.1.2">b</span>c</span>` +
		`<span class="koboSpan" id="kobo.1.3">d</span></p></body></html>`))
	require.NoError(t, err)
	require.Len(t, dt.spans, 2)
	assert.Equal(t, textSpan{id: spanID{para: 1, seg: 1}, start: 0, end: 3}, dt.spans[0])
	assert.Equal(t, textSpan{id: spanID{para: 1, seg: 3}, start: 3, end: 4}, dt.spans[1])
}
