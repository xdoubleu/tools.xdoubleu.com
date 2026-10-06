package services

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/net/html/charset"
)

// koboSpanType is the KoboLocation.Type of a span bookmark.
const koboSpanType = "KoboSpan"

// spanID is a koboSpan id, kobo.<para>.<seg>.
type spanID struct{ para, seg int32 }

func (id spanID) String() string {
	return "kobo." + strconv.Itoa(int(id.para)) + "." + strconv.Itoa(int(id.seg))
}

func parseSpanID(s string) (spanID, bool) {
	rest, ok := strings.CutPrefix(s, "kobo.")
	if !ok {
		return spanID{para: 0, seg: 0}, false
	}
	p, sg, ok := strings.Cut(rest, ".")
	if !ok {
		return spanID{para: 0, seg: 0}, false
	}
	para, err1 := strconv.ParseUint(p, 10, 31)
	seg, err2 := strconv.ParseUint(sg, 10, 31)
	if err1 != nil || err2 != nil {
		return spanID{para: 0, seg: 0}, false
	}
	return spanID{para: int32(para), seg: int32(seg)}, true
}

// spanStart is where a span's text starts in the original document's body
// text, in UTF-16 units.
type spanStart struct {
	id    spanID
	start int32
}

// docSpans is one content document's spans in document order; href is its
// zip-root path.
type docSpans struct {
	href  string
	spans []spanStart
}

// spanMap is a KEPUB's spans in original-EPUB offsets (ADR-0029).
type spanMap struct{ docs []docSpans }

// docText is a document's body text in UTF-16 units, plus its koboSpans'
// [start, end) in that text.
type docText struct {
	units []uint16
	spans []textSpan
}

type textSpan struct {
	id         spanID
	start, end int
}

// buildSpanMap maps every content document of kepub that the original EPUB
// also has. A document either side can't parse, or whose texts can't be
// aligned, is left out.
func buildSpanMap(ctx context.Context, kepub, original *zip.Reader) (*spanMap, error) {
	docs, err := epubContentDocs(kepub)
	if err != nil {
		return nil, err
	}

	m := &spanMap{docs: make([]docSpans, 0, len(docs))}
	for _, href := range docs {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		k, kErr := zipDocText(kepub, href)
		o, oErr := zipDocText(original, href)
		if kErr != nil || oErr != nil || len(k.spans) == 0 {
			continue
		}
		starts := make([]int, len(k.spans))
		for i, s := range k.spans {
			starts[i] = s.start
		}
		mapped, ok := alignStarts(ctx, k.units, o.units, starts)
		if !ok {
			continue
		}
		d := docSpans{href: href, spans: make([]spanStart, len(k.spans))}
		for i, s := range k.spans {
			start := int32(mapped[i]) //nolint:gosec // texts are under 1 GiB
			d.spans[i] = spanStart{id: s.id, start: start}
		}
		m.docs = append(m.docs, d)
	}
	return m, nil
}

func zipDocText(zr *zip.Reader, name string) (docText, error) {
	f, err := zr.Open(name)
	if err != nil {
		return docText{units: nil, spans: nil}, err
	}
	defer func() { _ = f.Close() }()
	return parseBodyText(f)
}

// epubContentDocs lists the OPF's XHTML content documents as zip-root paths,
// with kepubify's rules for which manifest items count.
func epubContentDocs(zr *zip.Reader) ([]string, error) {
	var container struct {
		RootFiles []struct {
			FullPath  string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeZipXML(zr, "META-INF/container.xml", &container); err != nil {
		return nil, err
	}
	opfPath := ""
	for _, rf := range container.RootFiles {
		if rf.MediaType == "application/oebps-package+xml" {
			opfPath = rf.FullPath
			break
		}
	}
	if opfPath == "" {
		return nil, errors.New("container.xml names no package document")
	}

	var opf struct {
		Items []struct {
			Href      string `xml:"href,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"manifest>item"`
	}
	if err := decodeZipXML(zr, opfPath, &opf); err != nil {
		return nil, err
	}

	var docs []string
	for _, it := range opf.Items {
		ext := strings.ToLower(path.Ext(it.Href))
		if it.MediaType != "application/xhtml+xml" && it.MediaType != "text/html" &&
			ext != ".xhtml" && ext != ".html" && ext != ".htm" {
			continue
		}
		href := it.Href
		if u, err := url.Parse(href); err == nil && u.Path != "" {
			href = u.Path
		}
		docs = append(docs, path.Join(path.Dir(opfPath), href))
	}
	return docs, nil
}

func decodeZipXML(zr *zip.Reader, name string, v any) error {
	f, err := zr.Open(name)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	d := xml.NewDecoder(f)
	d.CharsetReader = charset.NewReaderLabel
	if err = d.Decode(v); err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	return nil
}

// parseBodyText reads a document's body text the way the web reader counts
// body.textContent: every text and CDATA node under <body>, script and style
// included, entities resolved.
func parseBodyText(r io.Reader) (docText, error) {
	d := xml.NewDecoder(r)
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel

	b := &bodyTextReader{
		dt: docText{units: nil, spans: nil}, depth: 0, open: -1, openDepth: 0,
	}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return b.dt, nil
		}
		if err != nil {
			return docText{units: nil, spans: nil}, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			b.start(v)
		case xml.EndElement:
			if b.end() {
				return b.dt, nil
			}
		case xml.CharData:
			b.text(v)
		}
	}
}

// bodyTextReader accumulates a docText from XML tokens.
type bodyTextReader struct {
	dt        docText
	depth     int // element depth inside <body>; 0 is outside
	open      int // index of the koboSpan being read, -1 when none
	openDepth int
}

func (b *bodyTextReader) start(e xml.StartElement) {
	if b.depth == 0 {
		if e.Name.Local == "body" {
			b.depth = 1
		}
		return
	}
	b.depth++
	if id, ok := koboSpanID(e); ok && b.open < 0 {
		n := len(b.dt.units)
		b.dt.spans = append(b.dt.spans, textSpan{id: id, start: n, end: n})
		b.open, b.openDepth = len(b.dt.spans)-1, b.depth
	}
}

// end reports whether the body just closed.
func (b *bodyTextReader) end() bool {
	if b.depth == 0 {
		return false
	}
	if b.open >= 0 && b.depth == b.openDepth {
		b.dt.spans[b.open].end = len(b.dt.units)
		b.open = -1
	}
	b.depth--
	return b.depth == 0
}

func (b *bodyTextReader) text(c xml.CharData) {
	if b.depth == 0 {
		return
	}
	for _, r := range string(c) {
		b.dt.units = utf16.AppendRune(b.dt.units, r)
	}
}

func koboSpanID(e xml.StartElement) (spanID, bool) {
	if e.Name.Local != "span" {
		return spanID{para: 0, seg: 0}, false
	}
	var id string
	koboClass := false
	for _, a := range e.Attr {
		switch a.Name.Local {
		case "id":
			id = a.Value
		case "class":
			koboClass = strings.Contains(a.Value, "koboSpan")
		}
	}
	if !koboClass {
		return spanID{para: 0, seg: 0}, false
	}
	return parseSpanID(id)
}
