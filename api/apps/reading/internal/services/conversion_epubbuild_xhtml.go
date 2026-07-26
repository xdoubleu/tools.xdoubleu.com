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

// xhtmlNamespace is the namespace required on the root <html> element of an
// EPUB content document.
const xhtmlNamespace = "http://www.w3.org/1999/xhtml"

// disallowedElements are stripped entirely from the article body — EPUB
// readers reject them.
//
//nolint:gochecknoglobals // static lookup table
var disallowedElements = map[string]struct{}{
	"script": {},
	"iframe": {},
	"video":  {},
	"audio":  {},
	"form":   {},
}

// buildArticleXHTML parses htmlBytes, sanitizes the tree in place, collects
// the images it references (already downloaded as siblings of the source
// file by localizeImages), and serializes the result as XHTML.
func buildArticleXHTML(
	htmlBytes []byte, imgDir string,
) (string, []epubImage, error) {
	root, err := xhtml.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return "", nil, fmt.Errorf("parse input html: %w", err)
	}

	images := sanitizeAndCollectImages(root, imgDir)

	doc, err := renderXHTMLDocument(root)
	if err != nil {
		return "", nil, err
	}
	return doc, images, nil
}

// sanitizeAndCollectImages walks the parsed document, removing disallowed
// elements and unresolvable <img>s, stripping event-handler attributes from
// everything else, and returns the images to embed in document order.
// Mirrors the remove-while-iterating idiom in localizeImages
// (ingest_images.go).
func sanitizeAndCollectImages(root *xhtml.Node, imgDir string) []epubImage {
	var images []epubImage
	count := 0

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			switch {
			case child.Type != xhtml.ElementNode:
				// text/comment/etc. nodes: nothing to sanitize
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

// resolveArticleImage validates an <img> node's src against imgDir and
// returns the epubImage to embed. localizeImages normally rewrites src to a
// bare local filename, but falls back to the original (possibly remote or
// path-traversing) HTML if parsing fails during that earlier pass, so src is
// not guaranteed safe here and must be re-validated before it is used to
// open a file on disk.
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

// normalizeAttrs strips event-handler attributes and gives bare/boolean
// attributes an explicit value. x/net/html represents a true boolean
// attribute (e.g. <input disabled>) and an explicit empty value (e.g.
// alt="") identically, so this uniform rule is applied to every
// empty-valued attribute rather than an allowlist of known boolean names.
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

// renderXHTMLDocument serializes root (a full parsed document) as XHTML: an
// XML declaration followed by the <html> subtree with its namespace set.
// xhtml.Render already self-closes void elements and always quotes
// attribute values, so no custom serializer is needed here.
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

// findHTMLElement returns doc's <html> child. It only scans direct children
// (never recurses), so a nested element named "html" in the body can't be
// mismatched.
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
