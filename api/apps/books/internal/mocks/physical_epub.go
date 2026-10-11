package mocks

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// Content documents of PhysicalEPUB, as zip-root paths.
const (
	PhysicalFrontHref = "OEBPS/front.xhtml"
	PhysicalCh1Href   = "OEBPS/ch1.xhtml"
	PhysicalCh2Href   = "OEBPS/ch2.xhtml"
	PhysicalCh3Href   = "OEBPS/ch3.xhtml"
	// PhysicalSentences is the sentences per chapter.
	PhysicalSentences = 20
)

// PhysicalSentence is chapter ch's k-th sentence (1-based), including its
// trailing space.
func PhysicalSentence(ch, k int) string {
	return fmt.Sprintf("Chapter %d sentence %02d is right here. ", ch, k)
}

// PhysicalEPUB is an EPUB3 with front matter and three chapters of
// PhysicalSentences sentences. Its nav has a TOC (chapter two through an
// #c2 fragment after an epigraph) and a bodymatter landmark at chapter one.
// With printPages, page i marks the front matter and pages 1-6 start at each
// chapter and at its 11th sentence, listed in a page-list nav.
func PhysicalEPUB(title string, printPages bool) []byte {
	marker := func(label string) string {
		if !printPages {
			return ""
		}
		return fmt.Sprintf(`<span epub:type="pagebreak" id="page_%s" title="%s"/>`,
			label, label)
	}
	chapter := func(ch int, intro string) string {
		var b strings.Builder
		b.WriteString(intro + "<p>")
		for k := 1; k <= PhysicalSentences; k++ {
			if k == PhysicalSentences/2+1 {
				b.WriteString(marker(fmt.Sprint(2 * ch)))
			}
			b.WriteString(PhysicalSentence(ch, k))
		}
		b.WriteString("</p>")
		return physicalDoc(b.String())
	}

	pageList := ""
	if printPages {
		var b strings.Builder
		b.WriteString(`<nav epub:type="page-list"><ol>` +
			`<li><a href="front.xhtml#page_i">i</a></li>`)
		for p := 1; p <= 6; p++ {
			fmt.Fprintf(&b, `<li><a href="ch%d.xhtml#page_%d">%d</a></li>`, (p+1)/2, p, p)
		}
		b.WriteString(`</ol></nav>`)
		pageList = b.String()
	}

	files := []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>` +
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"` +
			` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"` +
			` media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"OEBPS/content.opf", physicalOPF(title)},
		{"OEBPS/nav.xhtml", physicalDoc(`<nav epub:type="toc"><ol>` +
			`<li><a href="ch1.xhtml">Chapter One</a></li>` +
			`<li><a href="ch2.xhtml#c2">Chapter  Two</a></li>` +
			`<li><a href="ch3.xhtml">Chapter Three</a></li></ol></nav>` +
			`<nav epub:type="landmarks"><ol>` +
			`<li><a epub:type="toc" href="nav.xhtml">Contents</a></li>` +
			`<li><a epub:type="bodymatter" href="ch1.xhtml">Start</a></li>` +
			`</ol></nav>` + pageList)},
		{PhysicalFrontHref, physicalDoc(marker("i") + "<p>Title page. Copyright.</p>")},
		{PhysicalCh1Href, chapter(1, marker("1"))},
		{PhysicalCh2Href, chapter(2, marker("3")+`<p>Epigraph.</p><h1 id="c2">Two</h1>`)},
		{PhysicalCh3Href, chapter(3, marker("5"))},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.content))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func physicalDoc(body string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Doc</title></head><body>` + body + "</body></html>\n"
}

func physicalOPF(title string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>%s</dc:title><dc:identifier id="id">physical-epub</dc:identifier>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="front" href="front.xhtml" media-type="application/xhtml+xml"/>
<item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="ch2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
<item id="ch3" href="ch3.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="front"/><itemref idref="ch1"/><itemref idref="ch2"/>` +
		`<itemref idref="ch3"/></spine>
</package>
`, title)
}
