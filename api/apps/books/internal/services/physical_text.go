package services

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf16"
)

// bookText is an EPUB's linear reading order, for mapping a physical copy's
// pages to neutral positions (ADR-0031). Offsets are body-text UTF-16 units,
// as in ADR-0029, and "global" offsets run across the spine.
type bookText struct {
	docs []textDoc
	// cum[i] is the global offset docs[i] starts at; cum[len(docs)] the total.
	cum []int
	// start is where the body matter starts.
	start int
	// printPages are the print edition's page starts, in reading order.
	printPages []printPage
	chapters   []bookChapter
}

type textDoc struct {
	href  string
	units []uint16
	// gaps are where block elements start or end, for spacing an excerpt;
	// body.textContent runs blocks together.
	gaps []int
}

type printPage struct {
	label string
	at    int
}

type bookChapter struct {
	title string
	at    int
}

// navLink is a navigation entry pointing at a zip-root document path and an
// optional fragment id.
type navLink struct {
	doc, frag, text, kind string
}

// Navigation kinds, from the EPUB3 nav's epub:type or the NCX's element.
const (
	navKindTOC       = "toc"
	navKindLandmarks = "landmarks"
	navKindPageList  = "page-list"
)

// buildBookText reads zr's linear spine documents, its navigation and its
// print page markers. A document that can't be read counts as empty.
func buildBookText(ctx context.Context, zr *zip.Reader) (*bookText, error) {
	opfPath, err := findOPFPath(zr)
	if err != nil {
		return nil, err
	}
	var pkg opfDoc
	if err = decodeZipXML(zr, opfPath, &pkg); err != nil {
		return nil, err
	}
	opfDir := path.Dir(opfPath)

	spine, index := spineDocs(pkg, opfDir)
	if len(spine) == 0 {
		return nil, errors.New("the spine has no content documents")
	}
	links := readNavigation(zr, pkg, opfDir)
	for _, g := range pkg.Guide {
		if strings.EqualFold(g.Type, "text") {
			doc, frag := resolveLink(opfPath, g.Href)
			links = append(links, navLink{doc: doc, frag: frag, text: "", kind: "guide-text"})
		}
	}

	wanted := map[string]map[string]bool{}
	for _, l := range links {
		if l.frag == "" {
			continue
		}
		if wanted[l.doc] == nil {
			wanted[l.doc] = map[string]bool{}
		}
		wanted[l.doc][l.frag] = true
	}

	t := &bookText{
		docs: make([]textDoc, len(spine)), cum: make([]int, len(spine)+1),
		start: 0, printPages: nil, chapters: nil,
	}
	scans := make([]docScan, len(spine))
	for i, href := range spine {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		scans[i] = scanZipDoc(zr, href, wanted[href])
		t.docs[i] = textDoc{href: href, units: scans[i].units, gaps: scans[i].gaps}
		t.cum[i+1] = t.cum[i] + len(scans[i].units)
	}

	at := func(l navLink) (int, bool) {
		i, ok := index[l.doc]
		if !ok {
			return 0, false
		}
		return t.cum[i] + scans[i].ids[l.frag], true
	}
	t.start = bodyMatterStart(links, at)
	t.chapters, t.printPages = linkTargets(links, at)
	if len(t.printPages) == 0 {
		for i, s := range scans {
			for _, p := range s.breaks {
				t.printPages = append(t.printPages, printPage{label: p.label, at: t.cum[i] + p.at})
			}
		}
	}
	sort.SliceStable(t.printPages, func(a, b int) bool {
		return t.printPages[a].at < t.printPages[b].at
	})
	return t, nil
}

// spineDocs lists the linear spine's content documents as zip-root paths,
// with each path's index.
func spineDocs(pkg opfDoc, opfDir string) ([]string, map[string]int) {
	items := make(map[string]int, len(pkg.Items))
	for i, it := range pkg.Items {
		items[it.ID] = i
	}
	var docs []string
	index := map[string]int{}
	for _, ref := range pkg.Spine.Refs {
		i, ok := items[ref.IDRef]
		if !ok || ref.Linear == "no" {
			continue
		}
		it := pkg.Items[i]
		if !isContentDoc(it.Href, it.MediaType) {
			continue
		}
		doc, _ := resolveLink(path.Join(opfDir, "x"), it.Href)
		if _, dup := index[doc]; dup {
			continue
		}
		index[doc] = len(docs)
		docs = append(docs, doc)
	}
	return docs, index
}

// bodyMatterStart is the EPUB3 bodymatter landmark, else the EPUB2 guide's
// text reference, else the start of the spine.
func bodyMatterStart(links []navLink, at func(navLink) (int, bool)) int {
	for _, kind := range []string{"bodymatter", "guide-text"} {
		for _, l := range links {
			if l.kind != kind {
				continue
			}
			if g, ok := at(l); ok {
				return g
			}
		}
	}
	return 0
}

// linkTargets resolves the TOC entries and the page list.
func linkTargets(
	links []navLink, at func(navLink) (int, bool),
) ([]bookChapter, []printPage) {
	var chapters []bookChapter
	var pages []printPage
	for _, l := range links {
		g, ok := at(l)
		if !ok {
			continue
		}
		switch l.kind {
		case navKindTOC:
			if l.text != "" {
				chapters = append(chapters, bookChapter{title: l.text, at: g})
			}
		case navKindPageList:
			if label := pageLabel(l.text); label != "" {
				pages = append(pages, printPage{label: label, at: g})
			}
		}
	}
	return chapters, pages
}

// resolveLink resolves href against the document at base into a zip-root
// path and fragment; an empty path is base itself.
func resolveLink(base, href string) (string, string) {
	u, err := url.Parse(href)
	if err != nil {
		return "", ""
	}
	if u.Path == "" {
		return base, u.Fragment
	}
	return path.Join(path.Dir(base), u.Path), u.Fragment
}

// readNavigation reads the EPUB3 nav document, else the NCX.
func readNavigation(zr *zip.Reader, pkg opfDoc, opfDir string) []navLink {
	var ncx string
	for _, it := range pkg.Items {
		doc, _ := resolveLink(path.Join(opfDir, "x"), it.Href)
		if hasToken(it.Properties, "nav") {
			if links := readZipNav(zr, doc, readNavDoc); len(links) > 0 {
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
	return readZipNav(zr, ncx, readNCX)
}

func readZipNav(
	zr *zip.Reader, doc string, read func(io.Reader, string) []navLink,
) []navLink {
	f, err := zr.Open(doc)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	return read(f, doc)
}

// readNavDoc collects the links of an EPUB3 nav document's toc, landmarks
// and page-list navs, with each link's text and, for landmarks, its type.
func readNavDoc(r io.Reader, doc string) []navLink {
	d := newHTMLDecoder(r)
	var links []navLink
	kind, navDepth, depth := "", 0, 0
	var open *navLink
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return links
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case v.Name.Local == "nav" && kind == "":
				kind, navDepth = navKind(epubType(v)), depth
			case v.Name.Local == "a" && kind != "" && open == nil:
				l := navLink{doc: "", frag: "", text: "", kind: kind}
				if kind == navKindLandmarks {
					l.kind = navKindLandmarks
					if hasToken(epubType(v), "bodymatter") {
						l.kind = "bodymatter"
					}
				}
				l.doc, l.frag = resolveLink(doc, attr(v, "href"))
				open = &l
				text.Reset()
			}
		case xml.EndElement:
			if open != nil && v.Name.Local == "a" {
				open.text = collapseSpace(text.String())
				if open.doc != "" {
					links = append(links, *open)
				}
				open = nil
			}
			if depth == navDepth {
				kind = ""
			}
			depth--
		case xml.CharData:
			if open != nil {
				text.Write(v)
			}
		}
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

// readNCX collects an EPUB2 NCX's navPoints as TOC links and its pageTargets
// as page-list links.
func readNCX(r io.Reader, doc string) []navLink {
	d := newHTMLDecoder(r)
	var links []navLink
	// labels is the open navPoints' and pageTargets' label text.
	var labels []*strings.Builder
	var kinds []string
	inText := false
	for {
		tok, err := d.Token()
		if err != nil {
			return links
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "navPoint", "pageTarget":
				labels = append(labels, &strings.Builder{})
				kind := navKindTOC
				if v.Name.Local == "pageTarget" {
					kind = navKindPageList
				}
				kinds = append(kinds, kind)
			case "text":
				inText = len(labels) > 0
			case "content":
				if len(labels) > 0 {
					target, frag := resolveLink(doc, attr(v, "src"))
					links = append(links, navLink{
						doc: target, frag: frag, kind: kinds[len(kinds)-1],
						text: collapseSpace(labels[len(labels)-1].String()),
					})
				}
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "navPoint", "pageTarget":
				if len(labels) > 0 {
					labels, kinds = labels[:len(labels)-1], kinds[:len(kinds)-1]
				}
			case "text":
				inText = false
			}
		case xml.CharData:
			if inText {
				labels[len(labels)-1].Write(v)
			}
		}
	}
}

// docScan is a document's body text, the offsets of the wanted ids, its
// print page markers and its block boundaries.
type docScan struct {
	units  []uint16
	ids    map[string]int
	breaks []printPage
	gaps   []int
}

// blockElements separate text visually though textContent runs it together.
//
//nolint:gochecknoglobals // constant set
var blockElements = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "li": true, "br": true, "blockquote": true,
	"section": true, "article": true, "header": true, "footer": true,
	"aside": true, "figure": true, "figcaption": true, "pre": true,
	"table": true, "tr": true, "td": true, "th": true, "dt": true, "dd": true,
	"hr": true,
}

// gap records a block boundary at the current offset.
func (s *docScan) gap() {
	if n := len(s.gaps); n == 0 || s.gaps[n-1] != len(s.units) {
		s.gaps = append(s.gaps, len(s.units))
	}
}

func scanZipDoc(zr *zip.Reader, name string, wanted map[string]bool) docScan {
	none := docScan{units: nil, ids: map[string]int{}, breaks: nil, gaps: nil}
	f, err := zr.Open(name)
	if err != nil {
		return none
	}
	defer func() { _ = f.Close() }()
	if info, statErr := f.Stat(); statErr == nil && info.Size() > maxContentDocBytes {
		return none
	}
	s, err := scanBookDoc(f, wanted)
	if err != nil {
		return none
	}
	return s
}

// scanBookDoc counts body text as parseBodyText does.
func scanBookDoc(r io.Reader, wanted map[string]bool) (docScan, error) {
	d := newHTMLDecoder(r)
	s := docScan{units: nil, ids: map[string]int{}, breaks: nil, gaps: nil}
	depth := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return s, nil
		}
		if err != nil {
			return docScan{units: nil, ids: map[string]int{}, breaks: nil, gaps: nil}, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if depth == 0 && v.Name.Local != "body" {
				continue
			}
			depth++
			if id := attr(v, "id"); wanted[id] {
				if _, seen := s.ids[id]; !seen {
					s.ids[id] = len(s.units)
				}
			}
			if label, ok := printPageMarker(v); ok {
				s.breaks = append(s.breaks, printPage{label: label, at: len(s.units)})
			}
			if blockElements[v.Name.Local] {
				s.gap()
			}
		case xml.EndElement:
			if depth == 0 {
				continue
			}
			if blockElements[v.Name.Local] {
				s.gap()
			}
			depth--
			if depth == 0 {
				return s, nil
			}
		case xml.CharData:
			if depth > 0 {
				for _, r := range string(v) {
					s.units = utf16.AppendRune(s.units, r)
				}
			}
		}
	}
}

// printPageMarker reads an epub:type="pagebreak" or role="doc-pagebreak"
// element's label from its title, aria-label or id.
func printPageMarker(e xml.StartElement) (string, bool) {
	marker := false
	var title, aria, id string
	for _, a := range e.Attr {
		switch {
		case a.Name.Local == "type" && a.Name.Space != "":
			marker = marker || hasToken(a.Value, "pagebreak")
		case a.Name.Local == "role":
			marker = marker || hasToken(a.Value, "doc-pagebreak")
		case a.Name.Local == "title":
			title = a.Value
		case a.Name.Local == "aria-label":
			aria = a.Value
		case a.Name.Local == "id":
			id = a.Value
		}
	}
	if !marker {
		return "", false
	}
	for _, raw := range []string{title, aria} {
		if label := pageLabel(raw); label != "" {
			return label, true
		}
	}
	digits := len(id)
	for digits > 0 && id[digits-1] >= '0' && id[digits-1] <= '9' {
		digits--
	}
	if digits == len(id) {
		return "", false
	}
	return id[digits:], true
}

// pageLabel is the last word of a page's label, so "Page 12" reads as "12".
func pageLabel(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// epubType is e's epub:type attribute.
func epubType(e xml.StartElement) string {
	for _, a := range e.Attr {
		if a.Name.Local == "type" && a.Name.Space != "" {
			return a.Value
		}
	}
	return ""
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}

func hasToken(list, token string) bool {
	for _, f := range strings.Fields(list) {
		if f == token {
			return true
		}
	}
	return false
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
