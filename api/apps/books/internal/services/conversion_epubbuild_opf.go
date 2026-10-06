package services

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func buildContainerXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(
		`<container version="1.0" ` +
			`xmlns="urn:oasis:names:tc:opendocument:xmlns:container">` + "\n",
	)
	b.WriteString("  <rootfiles>\n")
	b.WriteString(
		`    <rootfile full-path="OEBPS/content.opf" ` +
			`media-type="application/oebps-package+xml"/>` + "\n",
	)
	b.WriteString("  </rootfiles>\n")
	b.WriteString("</container>\n")
	return b.String()
}

// contentDocID is the manifest id of the i-th content document.
func contentDocID(i int) string {
	if i == 0 {
		return "doc"
	}
	return fmt.Sprintf("doc-%d", i)
}

// coverPage is the cover's content document; kepubify leaves a first spine
// item named like a cover alone instead of adding its own title page.
const coverPage = "cover.xhtml"

// coverImage returns meta's cover image as a manifest entry.
func coverImage(meta ArticleMeta) (epubImage, bool) {
	mediaType, ok := imageMediaTypes[strings.ToLower(filepath.Ext(meta.CoverImage))]
	if meta.CoverImage == "" || !ok {
		return epubImage{FileName: "", MediaType: "", ID: ""}, false
	}
	return epubImage{
		FileName:  meta.CoverImage,
		MediaType: mediaType,
		ID:        "cover-image",
	}, true
}

// buildCoverXHTML renders the cover page: the cover image alone.
func buildCoverXHTML(title, fileName string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(`<html xmlns="` + xhtmlNamespace + `">` + "\n")
	b.WriteString("<head><title>" + escapeXMLText(title) + "</title></head>\n")
	b.WriteString(
		`<body><img src="` + escapeXMLText(fileName) + `" alt="Cover"/></body>` + "\n",
	)
	b.WriteString("</html>\n")
	return b.String()
}

func buildContentOPF(
	meta ArticleMeta,
	images []epubImage,
	docs []contentDoc,
	cover epubImage,
	hasCover bool,
) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" ` +
			`unique-identifier="pub-id" xml:lang="en">` + "\n",
	)
	b.WriteString(
		`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n",
	)
	b.WriteString("    <dc:identifier id=\"pub-id\">urn:uuid:")
	if meta.Identifier != "" {
		b.WriteString(meta.Identifier)
	} else {
		// Only tests and callers without a stable identity reach this; a
		// per-book identity is required for the Kobo firmware to correlate
		// regenerated files with the book it already has (issue #1734).
		b.WriteString(uuid.NewString())
	}
	b.WriteString("</dc:identifier>\n")
	b.WriteString(
		"    <dc:title>" + escapeXMLText(meta.Title) + "</dc:title>\n",
	)
	for _, author := range meta.Authors {
		b.WriteString(
			"    <dc:creator>" + escapeXMLText(author) + "</dc:creator>\n",
		)
	}
	b.WriteString("    <dc:language>en</dc:language>\n")
	if hasCover {
		fmt.Fprintf(&b, "    <meta name=\"cover\" content=\"%s\"/>\n", cover.ID)
	}
	b.WriteString("  </metadata>\n")

	b.WriteString("  <manifest>\n")
	if hasCover {
		b.WriteString(`    <item id="cover" href="` + coverPage + `" ` +
			`media-type="application/xhtml+xml"/>` + "\n")
		fmt.Fprintf(
			&b,
			"    <item id=\"%s\" href=\"%s\" media-type=\"%s\" properties=\"cover-image\"/>\n",
			cover.ID,
			cover.FileName,
			cover.MediaType,
		)
	}
	for i, doc := range docs {
		fmt.Fprintf(&b,
			"    <item id=\"%s\" href=\"%s\" media-type=\"application/xhtml+xml\"/>\n",
			contentDocID(i), doc.Name)
	}
	b.WriteString(
		`    <item id="nav" href="nav.xhtml" ` +
			`media-type="application/xhtml+xml" properties="nav"/>` + "\n",
	)
	for _, img := range images {
		fmt.Fprintf(&b, "    <item id=\"%s\" href=\"%s\" media-type=\"%s\"/>\n",
			img.ID, img.FileName, img.MediaType)
	}
	b.WriteString("  </manifest>\n")

	b.WriteString("  <spine>\n")
	if hasCover {
		b.WriteString(`    <itemref idref="cover"/>` + "\n")
	}
	for i := range docs {
		fmt.Fprintf(&b, "    <itemref idref=\"%s\"/>\n", contentDocID(i))
	}
	b.WriteString("  </spine>\n")
	b.WriteString("</package>\n")

	return b.String()
}

// buildNavXHTML renders the EPUB nav document's TOC as a nested list of
// links to the toc entries (assignHeadingIDs), falling back to a single
// link to the whole book when the article has no TOC headings at all (a
// short feed article), then a converted PDF's page-list.
func buildNavXHTML(title string, toc, pages []tocEntry) string {
	escaped := escapeXMLText(title)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(
		`<html xmlns="http://www.w3.org/1999/xhtml" ` +
			`xmlns:epub="http://www.idpf.org/2007/ops">` + "\n",
	)
	b.WriteString("<head><title>" + escaped + "</title></head>\n")
	b.WriteString("<body>\n")
	b.WriteString(`  <nav epub:type="toc" id="toc">` + "\n")
	if len(toc) == 0 {
		b.WriteString("    <ol>\n")
		b.WriteString(
			`      <li><a href="index.xhtml">` + escaped + "</a></li>\n",
		)
		b.WriteString("    </ol>\n")
	} else {
		writeNavList(&b, toc)
	}
	b.WriteString("  </nav>\n")
	writePageList(&b, pages)
	b.WriteString("</body>\n")
	b.WriteString("</html>\n")
	return b.String()
}

// writeNavList writes toc as nested <ol> lists: a deeper entry opens a
// list inside the entry before it, a shallower one closes lists back to its
// level.
func writeNavList(b *strings.Builder, toc []tocEntry) {
	depth := 0
	for i, entry := range toc {
		level := entry.Level
		if i == 0 {
			level = 1
		}
		level = min(level, depth+1)
		switch {
		case level > depth:
			b.WriteString("<ol>")
		case level == depth:
			b.WriteString("</li>")
		default:
			for ; depth > level; depth-- {
				b.WriteString("</li></ol>")
			}
			b.WriteString("</li>")
		}
		depth = level
		file := entry.File
		if file == "" {
			file = contentDocName(0)
		}
		b.WriteString(`<li><a href="` + file + `#` + entry.ID + `">` +
			escapeXMLText(entry.Title) + "</a>")
	}
	for ; depth > 0; depth-- {
		b.WriteString("</li></ol>")
	}
	b.WriteString("\n")
}
