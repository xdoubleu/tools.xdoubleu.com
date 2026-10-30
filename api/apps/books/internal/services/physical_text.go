package services

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"net/url"
	"path"
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/books/pkg/ebooks"
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
	// kindBodyMatter and kindGuideText mark where the body matter starts.
	kindBodyMatter = "bodymatter"
	kindGuideText  = "guide-text"
)

// buildBookText reads zr's linear spine documents, its navigation and its
// print page markers. A document that can't be read counts as empty.
func buildBookText(ctx context.Context, zr *zip.Reader) (*bookText, error) {
	opfPath, err := ebooks.FindOPFPath(zr)
	if err != nil {
		return nil, err
	}
	var pkg ebooks.OPFDoc
	if err = ebooks.DecodeZipXML(zr, opfPath, &pkg); err != nil {
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
			links = append(links, navLink{doc: doc, frag: frag, text: "", kind: kindGuideText})
		}
	}

	wanted := wantedIDs(links)

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
		t.printPages = bodyPrintPages(scans, t.cum)
	}
	sort.SliceStable(t.printPages, func(a, b int) bool {
		return t.printPages[a].at < t.printPages[b].at
	})
	return t, nil
}

// wantedIDs is, per document, the fragment ids links point at.
func wantedIDs(links []navLink) map[string]map[string]bool {
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
	return wanted
}

// bodyPrintPages are the documents' page markers, for an EPUB without a page
// list.
func bodyPrintPages(scans []docScan, cum []int) []printPage {
	var pages []printPage
	for i, s := range scans {
		for _, p := range s.breaks {
			pages = append(pages, printPage{label: p.label, at: cum[i] + p.at})
		}
	}
	return pages
}

// spineDocs lists the linear spine's content documents as zip-root paths,
// with each path's index.
func spineDocs(pkg ebooks.OPFDoc, opfDir string) ([]string, map[string]int) {
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
		doc := ebooks.ResolveHref(opfDir, it.Href)
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
	for _, kind := range []string{kindBodyMatter, kindGuideText} {
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
