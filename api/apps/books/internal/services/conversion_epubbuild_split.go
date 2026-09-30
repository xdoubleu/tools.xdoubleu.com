package services

import (
	"fmt"

	xhtml "golang.org/x/net/html"
)

// splitMaxLevel: the body splits into a new content document at every TOC
// heading this deep or shallower (parts and chapters, not sections).
const splitMaxLevel = 2

// contentDoc is one XHTML content document of the book.
type contentDoc struct {
	Name  string
	XHTML string
}

// contentDocName names the i-th content document.
func contentDocName(i int) string {
	if i == 0 {
		return "index.xhtml"
	}
	return fmt.Sprintf("index-%d.xhtml", i)
}

// splitContentDocs serializes root as one content document per chapter:
// the body's top-level nodes split at each TOC heading of splitMaxLevel or
// shallower, with anything before the first in a document of its own.
// Kobo lays out and paginates a whole content document at once, so one
// document per chapter keeps a long book responsive and gives its chapter
// progress real boundaries. toc entries get the document they land in.
func splitContentDocs(root *xhtml.Node, toc []tocEntry) ([]contentDoc, error) {
	body := findElement(findHTMLElement(root), "body")
	if body == nil {
		doc, err := renderXHTMLDocument(root)
		return []contentDoc{{Name: contentDocName(0), XHTML: doc}}, err
	}

	splitAt := map[string]bool{}
	for _, e := range toc {
		if e.Level <= splitMaxLevel {
			splitAt[e.ID] = true
		}
	}
	var chunks [][]*xhtml.Node
	var cur []*xhtml.Node
	hasContent := false
	for n := body.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xhtml.ElementNode && splitAt[attrValue(n, "id")] && hasContent {
			chunks = append(chunks, cur)
			cur, hasContent = nil, false
		}
		cur = append(cur, n)
		hasContent = hasContent || n.Type == xhtml.ElementNode
	}
	chunks = append(chunks, cur)

	for _, n := range append([]*xhtml.Node(nil), nodesOf(body)...) {
		body.RemoveChild(n)
	}
	docs := make([]contentDoc, len(chunks))
	fileOf := map[string]string{}
	for i, chunk := range chunks {
		for _, n := range chunk {
			body.AppendChild(n)
			for _, id := range idsWithin(n) {
				fileOf[id] = contentDocName(i)
			}
		}
		doc, err := renderXHTMLDocument(root)
		if err != nil {
			return nil, err
		}
		docs[i] = contentDoc{Name: contentDocName(i), XHTML: doc}
		for _, n := range chunk {
			body.RemoveChild(n)
		}
	}
	for i := range toc {
		toc[i].File = fileOf[toc[i].ID]
	}
	return docs, nil
}

func nodesOf(parent *xhtml.Node) []*xhtml.Node {
	var nodes []*xhtml.Node
	for n := parent.FirstChild; n != nil; n = n.NextSibling {
		nodes = append(nodes, n)
	}
	return nodes
}

// idsWithin returns the id attributes of n and its descendants.
func idsWithin(n *xhtml.Node) []string {
	var ids []string
	if n.Type == xhtml.ElementNode {
		if id := attrValue(n, "id"); id != "" {
			ids = append(ids, id)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ids = append(ids, idsWithin(c)...)
	}
	return ids
}

// findElement returns parent's first child element named tag.
func findElement(parent *xhtml.Node, tag string) *xhtml.Node {
	if parent == nil {
		return nil
	}
	for n := parent.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xhtml.ElementNode && n.Data == tag {
			return n
		}
	}
	return nil
}
