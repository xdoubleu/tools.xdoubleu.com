package services

import (
	"fmt"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

const (
	// pageAnchorPrefix starts the id of the anchor where a PDF page begins:
	// pdfpage-N, N 1-based.
	pageAnchorPrefix = "pdfpage-"
	// epubNamespace is the namespace of epub:type attributes.
	epubNamespace = "http://www.idpf.org/2007/ops"
)

// withPageAnchor is blk's html with the anchor of the page it opens, if any:
// inside its element, so it stays with the block when a heading starts a new
// content document, or before a void element.
func withPageAnchor(blk htmlBlock) string {
	if blk.page <= 0 {
		return blk.html
	}
	anchor := pageAnchorHTML(blk.page)
	end := strings.IndexByte(blk.html, '>')
	if blk.tag == imgTag || end <= 0 || blk.html[end-1] == '/' {
		return anchor + blk.html
	}
	return blk.html[:end+1] + anchor + blk.html[end+1:]
}

// pageAnchorHTML marks where PDF page starts.
func pageAnchorHTML(page int) string {
	return fmt.Sprintf(
		`<span epub:type="pagebreak" id="%s%d" role="doc-pagebreak"></span>`,
		pageAnchorPrefix, page,
	)
}

// pageListEntries returns the page anchors in document order as page-list
// entries labelled with their page number.
func pageListEntries(root *xhtml.Node) []tocEntry {
	anchors := headingsWhere(root, func(n *xhtml.Node) bool {
		return n.Data == "span" && attrValue(n, "epub:type") == "pagebreak" &&
			strings.HasPrefix(attrValue(n, "id"), pageAnchorPrefix)
	})
	entries := make([]tocEntry, 0, len(anchors))
	for _, n := range anchors {
		id := attrValue(n, "id")
		page, err := strconv.Atoi(strings.TrimPrefix(id, pageAnchorPrefix))
		if err != nil || page < 1 {
			continue
		}
		entries = append(entries, tocEntry{
			ID: id, Title: strconv.Itoa(page), Level: 1, File: "",
		})
	}
	return entries
}

// writePageList writes the nav document's page-list, hidden as EPUB 3
// recommends; nothing when the book has no page anchors.
func writePageList(b *strings.Builder, pages []tocEntry) {
	if len(pages) == 0 {
		return
	}
	b.WriteString(`  <nav epub:type="page-list" hidden="hidden"><ol>` + "\n")
	for _, p := range pages {
		file := p.File
		if file == "" {
			file = contentDocName(0)
		}
		b.WriteString(`<li><a href="` + file + `#` + p.ID + `">` +
			escapeXMLText(p.Title) + "</a></li>\n")
	}
	b.WriteString("</ol></nav>\n")
}
