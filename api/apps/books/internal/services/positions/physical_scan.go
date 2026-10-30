package positions

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"unicode/utf16"
)

// docScan is a document's body text, the offsets of the wanted ids, its
// print page markers and its block boundaries.
type docScan struct {
	units  []uint16
	ids    map[string]int
	breaks []printPage
	gaps   []int
}

func emptyDocScan() docScan {
	return docScan{units: nil, ids: map[string]int{}, breaks: nil, gaps: nil}
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
	f, err := zr.Open(name)
	if err != nil {
		return emptyDocScan()
	}
	defer func() { _ = f.Close() }()
	if info, statErr := f.Stat(); statErr == nil && info.Size() > maxContentDocBytes {
		return emptyDocScan()
	}
	s, err := scanBookDoc(f, wanted)
	if err != nil {
		return emptyDocScan()
	}
	return s
}

// scanBookDoc counts body text as parseBodyText does.
func scanBookDoc(r io.Reader, wanted map[string]bool) (docScan, error) {
	sc := &docScanner{scan: emptyDocScan(), wanted: wanted, depth: 0}
	if err := readTokens(newHTMLDecoder(r), sc); err != nil {
		return emptyDocScan(), err
	}
	return sc.scan, nil
}

type docScanner struct {
	scan   docScan
	wanted map[string]bool
	// depth is the element depth inside <body>; 0 is outside.
	depth int
}

func (sc *docScanner) start(e xml.StartElement) {
	if sc.depth == 0 && e.Name.Local != tagBody {
		return
	}
	sc.depth++
	s := &sc.scan
	if id := attr(e, "id"); sc.wanted[id] {
		if _, seen := s.ids[id]; !seen {
			s.ids[id] = len(s.units)
		}
	}
	if label, ok := printPageMarker(e); ok {
		s.breaks = append(s.breaks, printPage{label: label, at: len(s.units)})
	}
	if blockElements[e.Name.Local] {
		s.gap()
	}
}

func (sc *docScanner) end(e xml.EndElement) bool {
	if sc.depth == 0 {
		return false
	}
	if blockElements[e.Name.Local] {
		sc.scan.gap()
	}
	sc.depth--
	return sc.depth == 0
}

func (sc *docScanner) text(c xml.CharData) {
	if sc.depth == 0 {
		return
	}
	for _, r := range string(c) {
		sc.scan.units = utf16.AppendRune(sc.scan.units, r)
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
