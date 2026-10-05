package mocks

import (
	"archive/zip"
	"bytes"
	"fmt"
)

// Content documents of ChapterEPUB, as zip-root paths.
const (
	ChapterOneHref   = "OEBPS/Text/ch1.xhtml"
	ChapterTwoHref   = "OEBPS/Text/ch2.xhtml"
	ChapterThreeHref = "OEBPS/Text/ch3.xhtml"
)

// ChapterTwoText is ChapterTwoHref's body text; kepubify spans it as kobo.1.1
// "Second chapter starts here. ", kobo.1.2 "It continues.", kobo.2.1
// "Another paragraph.".
const ChapterTwoText = "Second chapter starts here. It continues.Another paragraph."

const xhtmlHead = `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"` +
	` "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml"` +
	` xmlns:o="urn:schemas-microsoft-com:office:office">
<head><title>Chapter</title><style type="text/css">p { margin: 0 }</style></head>
`

// chapterOneBody is ordinary prose: entities, inline markup, lists, tables,
// an image, script/style, SVG text and CRLF line endings.
const chapterOneBody = "<body>\r\n<h1>Chapter One</h1>\r\n" +
	`<p>It was a dark night. The wind howled&nbsp;&mdash; loudly! Did it stop?` +
	` No.</p>
<p>Caf&#233; &amp; <em>bar</em> &#x1F600; emoji&hellip; &#8220;quoted.&#8221; Next.</p>
<ul>
<li>First item.</li>
<li>Second item.</li>
</ul>
<table>
<tr><td>Cell one.</td><td>Cell two.</td></tr>
</table>
<p>Before image <img src="../Images/a.png" alt="a"/> after image.</p>
<script type="text/javascript">var x = 1 &lt; 2;</script>
<style type="text/css">p { color: black }</style>
<svg xmlns="http://www.w3.org/2000/svg"><text>Vector text</text></svg>
<div><p>Line one<br/>Line two.</p></div>
  <!-- a comment -->
</body>
`

// chapterThreeBody holds every construct whose text kepubify's HTML5 parser
// changes.
const chapterThreeBody = `<body>
<pre>
code line
</pre>
<p>After pre. Text<![CDATA[ raw ]]> more.</p>
<p>Word doc<o:p> </o:p> tail.</p>
<noscript><p>No script.</p></noscript>
<p>Bad ` + "�" + ` char. End.</p>
<textarea>
area</textarea>
<p>Final sentence.</p>
</body>
`

// ChapterEPUB returns a three-chapter EPUB whose OPF matches title/author.
func ChapterEPUB(title, author string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>` +
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"` +
			` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"` +
			` media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"OEBPS/content.opf", chapterOPF(title, author)},
		{ChapterOneHref, xhtmlHead + chapterOneBody + "</html>\n"},
		{ChapterTwoHref, xhtmlHead + "<body><p>Second chapter starts here." +
			" It continues.</p><p>Another paragraph.</p></body>\n</html>\n"},
		{ChapterThreeHref, xhtmlHead + chapterThreeBody + "</html>\n"},
	}
	for _, f := range files {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.content))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func chapterOPF(title, author string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>%s</dc:title><dc:creator>%s</dc:creator>
<dc:identifier id="id">chapter-epub</dc:identifier>
</metadata>
<manifest>
<item id="ch1" href="Text/ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="ch2" href="Text/ch2.xhtml" media-type="application/xhtml+xml"/>
<item id="ch3" href="Text/ch3.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="ch1"/><itemref idref="ch2"/><itemref idref="ch3"/></spine>
</package>
`, title, author)
}
