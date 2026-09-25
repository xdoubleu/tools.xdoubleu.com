package services

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	xhtml "golang.org/x/net/html"
)

const xhtmlNamespace = "http://www.w3.org/1999/xhtml"

// disallowedElements are stripped entirely; EPUB readers reject them.
//
//nolint:gochecknoglobals // static lookup table
var disallowedElements = map[string]struct{}{
	"script": {},
	"iframe": {},
	"video":  {},
	"audio":  {},
	"form":   {},
}

// tocEntry is one <h1> chapter entry in nav.xhtml.
type tocEntry struct {
	ID    string
	Title string
}

// buildArticleXHTML sanitizes htmlBytes, collects its (already localized)
// images, assigns <h1> anchor ids for the TOC, and serializes as XHTML.
func buildArticleXHTML(
	htmlBytes []byte, imgDir string,
) (string, []epubImage, []tocEntry, error) {
	root, err := xhtml.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse input html: %w", err)
	}

	images := sanitizeAndCollectImages(root, imgDir)
	toc := assignHeadingIDs(root)

	doc, err := renderXHTMLDocument(root)
	if err != nil {
		return "", nil, nil, err
	}
	return doc, images, toc, nil
}

// assignHeadingIDs gives every <h1> a "heading-N" id and returns the TOC
// entries, so nav.xhtml can link to chapters.
func assignHeadingIDs(root *xhtml.Node) []tocEntry {
	var entries []tocEntry
	count := 0

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xhtml.ElementNode && child.Data == "h1" {
				id := fmt.Sprintf("heading-%d", count)
				count++
				setAttr(child, "id", id)
				entries = append(
					entries, tocEntry{ID: id, Title: textContent(child)},
				)
			}
			walk(child)
		}
	}
	walk(root)

	return entries
}

func textContent(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

// sanitizeAndCollectImages removes disallowed elements and unresolvable
// <img>s, strips event handlers, and returns the images in document order.
func sanitizeAndCollectImages(root *xhtml.Node, imgDir string) []epubImage {
	var images []epubImage
	count := 0

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			switch {
			case child.Type != xhtml.ElementNode:
			case isDisallowedElement(child.Data):
				n.RemoveChild(child)
			case child.Data == imgTag:
				normalizeAttrs(child)
				if img, ok := resolveArticleImage(child, imgDir, count); ok {
					images = append(images, img)
					count++
				} else {
					n.RemoveChild(child)
				}
			default:
				normalizeAttrs(child)
				walk(child)
			}
			child = next
		}
	}
	walk(root)

	return images
}

func isDisallowedElement(tag string) bool {
	_, ok := disallowedElements[tag]
	return ok
}

// resolveArticleImage re-validates src against imgDir: localizeImages falls
// back to the original (possibly remote or path-traversing) HTML on a parse
// failure, so src isn't guaranteed safe.
func resolveArticleImage(
	node *xhtml.Node, imgDir string, index int,
) (epubImage, bool) {
	noImage := epubImage{FileName: "", MediaType: "", ID: ""}

	src := attrValue(node, srcAttr)
	if src == "" || strings.Contains(src, "://") || filepath.IsAbs(src) {
		return noImage, false
	}

	cleanDir := filepath.Clean(imgDir)
	full := filepath.Clean(filepath.Join(cleanDir, src))
	if full != cleanDir &&
		!strings.HasPrefix(full, cleanDir+string(filepath.Separator)) {
		return noImage, false
	}

	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return noImage, false
	}

	mediaType, ok := imageMediaTypes[strings.ToLower(filepath.Ext(src))]
	if !ok {
		return noImage, false
	}

	return epubImage{
		FileName:  src,
		MediaType: mediaType,
		ID:        fmt.Sprintf("item-img%d", index),
	}, true
}

// normalizeAttrs strips event handlers and gives every empty attribute an
// explicit value: x/net/html can't tell a boolean attribute from alt="".
func normalizeAttrs(node *xhtml.Node) {
	out := node.Attr[:0]
	for _, a := range node.Attr {
		if isEventAttr(a.Key) {
			continue
		}
		if a.Val == "" {
			a.Val = a.Key
		}
		out = append(out, a)
	}
	node.Attr = out
}

func isEventAttr(key string) bool {
	return len(key) > 2 && strings.EqualFold(key[:2], "on")
}

func attrValue(node *xhtml.Node, key string) string {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// renderXHTMLDocument prefixes an XML declaration and sets the namespace;
// xhtml.Render already self-closes voids and quotes values.
func renderXHTMLDocument(root *xhtml.Node) (string, error) {
	htmlEl := findHTMLElement(root)
	if htmlEl == nil {
		return "", errors.New("parsed document has no html element")
	}
	setAttr(htmlEl, "xmlns", xhtmlNamespace)

	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	if err := xhtml.Render(&buf, htmlEl); err != nil {
		return "", fmt.Errorf("render xhtml: %w", err)
	}
	return buf.String(), nil
}

// findHTMLElement scans only doc's direct children, so a nested "html" can't match.
func findHTMLElement(doc *xhtml.Node) *xhtml.Node {
	for n := doc.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xhtml.ElementNode && n.Data == "html" {
			return n
		}
	}
	return nil
}

func setAttr(node *xhtml.Node, key, val string) {
	for i, a := range node.Attr {
		if a.Key == key {
			node.Attr[i].Val = val
			return
		}
	}
	node.Attr = append(
		node.Attr, xhtml.Attribute{Namespace: "", Key: key, Val: val},
	)
}
