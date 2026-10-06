package mocks

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// Content documents of PagedEPUB, as zip-root paths.
const (
	PagedDocOne = "OEBPS/index.xhtml"
	PagedDocTwo = "OEBPS/index-1.xhtml"
)

// PagedEPUB is an EPUB like the PDF converter's output for a 3-page PDF:
// pages 1 and 2 in PagedDocOne, page 3 opening PagedDocTwo, each marked by a
// pdfpage-N anchor. intro extra paragraphs on page 1 shift the spans after
// them, as a re-conversion can. kepubify spans PagedDocOne's paragraphs
// kobo.1.1, kobo.2.1, …, page 2's being kobo.(3+intro).1, and PagedDocTwo's
// kobo.1.1 "Chapter Two", kobo.2.1 "Page three body.".
func PagedEPUB(title, author string, intro int) []byte {
	anchor := func(n int) string {
		return fmt.Sprintf(`<span epub:type="pagebreak" id="pdfpage-%d"`+
			` role="doc-pagebreak"></span>`, n)
	}
	head := `<?xml version="1.0" encoding="utf-8"?>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"` +
		` xmlns:epub="http://www.idpf.org/2007/ops"><head><title>P</title></head>`
	const pageOne, pageTwo, pageThree = 1, 2, 3
	docOne := head + "<body><p>" + anchor(pageOne) + "Page one text.</p>" +
		"<p>More of page one.</p>" +
		strings.Repeat("<p>Intro paragraph.</p>", intro) +
		"<p>" + anchor(pageTwo) + "Page two text.</p></body></html>"
	docTwo := head + "<body><h1>" + anchor(pageThree) + "Chapter Two</h1>" +
		"<p>Page three body.</p></body></html>"

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>` +
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"` +
			` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"` +
			` media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"OEBPS/content.opf", pagedOPF(title, author)},
		{PagedDocOne, docOne},
		{PagedDocTwo, docTwo},
	} {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.content))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func pagedOPF(title, author string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>%s</dc:title><dc:creator>%s</dc:creator>
<dc:identifier id="id">paged-epub</dc:identifier>
</metadata>
<manifest>
<item id="doc" href="index.xhtml" media-type="application/xhtml+xml"/>
<item id="doc-1" href="index-1.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="doc"/><itemref idref="doc-1"/></spine>
</package>
`, title, author)
}
