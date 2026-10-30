package services

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"tools.xdoubleu.com/apps/books/pkg/ebooks"
)

const (
	tagBody       = "body"
	tagPageTarget = "pageTarget"
	tagNavPoint   = "navPoint"
)

// tokenHandler consumes an XML token stream.
type tokenHandler interface {
	start(e xml.StartElement)
	// end reports whether the handler is done.
	end(e xml.EndElement) bool
	text(c xml.CharData)
}

// readTokens feeds d's tokens to h until the end, h is done, or an error.
// CharData is only valid during the call.
func readTokens(d *xml.Decoder, h tokenHandler) error {
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			h.start(v)
		case xml.EndElement:
			if h.end(v) {
				return nil
			}
		case xml.CharData:
			h.text(v)
		}
	}
}

// readNavigation reads the EPUB3 nav document, else the NCX.
func readNavigation(zr *zip.Reader, pkg ebooks.OPFDoc, opfDir string) []navLink {
	var ncx string
	for _, it := range pkg.Items {
		doc := ebooks.ResolveHref(opfDir, it.Href)
		if hasToken(it.Properties, "nav") {
			if links := readZipNav(zr, doc, newNavDocReader(doc)); len(links) > 0 {
				return links
			}
		}
		if it.ID == pkg.Spine.TOC || it.MediaType == "application/x-dtbncx+xml" {
			ncx = doc
		}
	}
	if ncx == "" {
		return nil
	}
	return readZipNav(zr, ncx, newNCXReader(ncx))
}

// navReader is a tokenHandler collecting links.
type navReader interface {
	tokenHandler
	result() []navLink
}

// readZipNav reads doc with r; a document that fails part-way keeps the
// links read so far.
func readZipNav(zr *zip.Reader, doc string, r navReader) []navLink {
	f, err := zr.Open(doc)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	_ = readTokens(newHTMLDecoder(f), r)
	return r.result()
}

// navDocReader collects the links of an EPUB3 nav document's toc, landmarks
// and page-list navs, with each link's text and, for landmarks, its type.
type navDocReader struct {
	doc             string
	links           []navLink
	kind            string
	depth, navDepth int
	open            *navLink
	label           strings.Builder
}

func newNavDocReader(doc string) *navDocReader {
	return &navDocReader{
		doc: doc, links: nil, kind: "", depth: 0, navDepth: 0, open: nil,
		label: strings.Builder{},
	}
}

func (r *navDocReader) result() []navLink { return r.links }

func (r *navDocReader) start(e xml.StartElement) {
	r.depth++
	switch {
	case e.Name.Local == "nav" && r.kind == "":
		r.kind, r.navDepth = navKind(epubType(e)), r.depth
	case e.Name.Local == "a" && r.kind != "" && r.open == nil:
		l := navLink{doc: "", frag: "", text: "", kind: r.kind}
		if r.kind == navKindLandmarks && hasToken(epubType(e), "bodymatter") {
			l.kind = kindBodyMatter
		}
		l.doc, l.frag = resolveLink(r.doc, attr(e, "href"))
		r.open = &l
		r.label.Reset()
	}
}

func (r *navDocReader) end(e xml.EndElement) bool {
	if r.open != nil && e.Name.Local == "a" {
		r.open.text = collapseSpace(r.label.String())
		if r.open.doc != "" {
			r.links = append(r.links, *r.open)
		}
		r.open = nil
	}
	if r.depth == r.navDepth {
		r.kind = ""
	}
	r.depth--
	return false
}

func (r *navDocReader) text(c xml.CharData) {
	if r.open != nil {
		r.label.Write(c)
	}
}

func navKind(types string) string {
	for _, k := range []string{navKindTOC, navKindLandmarks, navKindPageList} {
		if hasToken(types, k) {
			return k
		}
	}
	return ""
}

// ncxReader collects an EPUB2 NCX's navPoints as TOC links and its
// pageTargets as page-list links.
type ncxReader struct {
	doc   string
	links []navLink
	// labels and kinds are the open navPoints' and pageTargets'.
	labels []*strings.Builder
	kinds  []string
	inText bool
}

func newNCXReader(doc string) *ncxReader {
	return &ncxReader{doc: doc, links: nil, labels: nil, kinds: nil, inText: false}
}

func (r *ncxReader) result() []navLink { return r.links }

func (r *ncxReader) start(e xml.StartElement) {
	switch e.Name.Local {
	case tagNavPoint, tagPageTarget:
		r.labels = append(r.labels, &strings.Builder{})
		kind := navKindTOC
		if e.Name.Local == tagPageTarget {
			kind = navKindPageList
		}
		r.kinds = append(r.kinds, kind)
	case "text":
		r.inText = len(r.labels) > 0
	case "content":
		if n := len(r.labels); n > 0 {
			target, frag := resolveLink(r.doc, attr(e, "src"))
			r.links = append(r.links, navLink{
				doc: target, frag: frag, kind: r.kinds[n-1],
				text: collapseSpace(r.labels[n-1].String()),
			})
		}
	}
}

func (r *ncxReader) end(e xml.EndElement) bool {
	switch e.Name.Local {
	case tagNavPoint, tagPageTarget:
		// The decoder only ends an element it started, so one is open.
		n := len(r.labels) - 1
		r.labels, r.kinds = r.labels[:n], r.kinds[:n]
	case "text":
		r.inText = false
	}
	return false
}

func (r *ncxReader) text(c xml.CharData) {
	if r.inText {
		r.labels[len(r.labels)-1].Write(c)
	}
}
