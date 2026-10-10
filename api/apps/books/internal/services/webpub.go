package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"
	xhtml "golang.org/x/net/html"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// ErrWebPubResourceNotFound is returned when a requested path isn't in the EPUB.
var ErrWebPubResourceNotFound = errors.New("webpub resource not found")

const (
	webPubContext   = "https://readium.org/webpub-manifest/context.jsonld"
	webPubEPUBConf  = "https://readium.org/webpub-manifest/profiles/epub"
	mimeXHTML       = "application/xhtml+xml"
	mimePositions   = "application/vnd.readium.position-list+json"
	positionsHref   = "positions.json"
	positionLength  = 1024
	opfMediaType    = "application/oebps-package+xml"
	maxWebPubEntry  = 100 << 20
	resourceBaseDir = "res/"
	tagSpan         = "span"
)

// WebPubLink is one Readium link object.
type WebPubLink struct {
	Href       string         `json:"href"`
	Type       string         `json:"type,omitempty"`
	Title      string         `json:"title,omitempty"`
	Rel        []string       `json:"rel,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Children   []WebPubLink   `json:"children,omitempty"`
}

// WebPubMetadata is the subset of Readium metadata the reader needs.
type WebPubMetadata struct {
	Title      string   `json:"title"`
	Author     []string `json:"author,omitempty"`
	Language   []string `json:"language,omitempty"`
	Identifier string   `json:"identifier,omitempty"`
	ConformsTo string   `json:"conformsTo"`
}

// WebPubManifest is a Readium Web Publication Manifest. Hrefs are relative to
// the manifest URL: resources live under res/<zip path>.
type WebPubManifest struct {
	Context      string         `json:"@context"`
	Metadata     WebPubMetadata `json:"metadata"`
	Links        []WebPubLink   `json:"links"`
	ReadingOrder []WebPubLink   `json:"readingOrder"`
	Resources    []WebPubLink   `json:"resources"`
	TOC          []WebPubLink   `json:"toc,omitempty"`
}

// WebPubLocations places a position within its section and the whole book.
type WebPubLocations struct {
	Position         int     `json:"position"`
	Progression      float64 `json:"progression"`
	TotalProgression float64 `json:"totalProgression"`
}

// WebPubLocator is one entry of a Readium position list.
type WebPubLocator struct {
	Href      string          `json:"href"`
	Type      string          `json:"type"`
	Locations WebPubLocations `json:"locations"`
}

// WebPubPositions is a Readium position list. The EPUB navigator needs it to
// place any locator, so a manifest without one renders nothing.
type WebPubPositions struct {
	Total     int             `json:"total"`
	Positions []WebPubLocator `json:"positions"`
}

// WebPubResource is one entry streamed out of the EPUB zip.
type WebPubResource struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// WebPubService derives Readium manifests from stored EPUBs and streams single
// zip entries. It reads only stored bytes, never a user-supplied URL.
type WebPubService struct {
	books *BookService
}

func newWebPubService(books *BookService) *WebPubService {
	return &WebPubService{books: books}
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfDoc struct {
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Languages   []string `xml:"language"`
		Identifiers []string `xml:"identifier"`
	} `xml:"metadata"`
	Items []opfItem `xml:"manifest>item"`
	Spine struct {
		TOC  string `xml:"toc,attr"`
		Refs []struct {
			IDRef  string `xml:"idref,attr"`
			Linear string `xml:"linear,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// Manifest builds the manifest for the user's original EPUB of bookID.
func (s *WebPubService) Manifest(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*WebPubManifest, error) {
	zr, err := s.openEPUB(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	return buildWebPubManifest(zr)
}

// Positions builds the position list for the user's original EPUB of bookID.
func (s *WebPubService) Positions(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*WebPubPositions, error) {
	zr, err := s.openEPUB(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	m, err := buildWebPubManifest(zr)
	if err != nil {
		return nil, err
	}
	return buildWebPubPositions(zr, m), nil
}

// Resource opens one zip entry of the user's original EPUB.
func (s *WebPubService) Resource(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	name string,
) (*WebPubResource, error) {
	name = path.Clean(strings.TrimPrefix(name, "/"))
	if name == "." || strings.HasPrefix(name, "..") {
		return nil, ErrWebPubResourceNotFound
	}
	zr, err := s.openEPUB(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > maxWebPubEntry {
			return nil, ErrWebPubResourceNotFound
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return nil, fmt.Errorf("open %s: %w", name, openErr)
		}
		return &WebPubResource{
			Body:        rc,
			ContentType: resourceType(name),
			Size:        int64(f.UncompressedSize64),
		}, nil
	}
	return nil, ErrWebPubResourceNotFound
}

func (s *WebPubService) openEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*zip.Reader, error) {
	file, err := s.books.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatEPUB,
	)
	if err != nil {
		return nil, err
	}
	if file.Status != models.FileStatusReady {
		return nil, database.ErrResourceNotFound
	}
	rc, err := s.books.objectStore.Get(ctx, file.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("get epub: %w", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read epub: %w", err)
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

func buildWebPubManifest(zr *zip.Reader) (*WebPubManifest, error) {
	opfPath, err := findOPFPath(zr)
	if err != nil {
		return nil, err
	}
	var opf opfDoc
	if err = decodeZipXML(zr, opfPath, &opf); err != nil {
		return nil, err
	}
	base := path.Dir(opfPath)
	byID := make(map[string]opfItem, len(opf.Items))
	for _, it := range opf.Items {
		byID[it.ID] = it
	}

	m := &WebPubManifest{
		Context: webPubContext,
		Metadata: WebPubMetadata{
			Title:      first(opf.Metadata.Titles),
			Author:     opf.Metadata.Creators,
			Language:   opf.Metadata.Languages,
			Identifier: first(opf.Metadata.Identifiers),
			ConformsTo: webPubEPUBConf,
		},
		Links: []WebPubLink{
			newLink("manifest.json", "application/webpub+json", "self"),
			newLink(positionsHref, mimePositions),
		},
		ReadingOrder: []WebPubLink{},
		Resources:    []WebPubLink{},
		TOC:          nil,
	}

	inSpine := map[string]bool{}
	for _, ref := range opf.Spine.Refs {
		it, ok := byID[ref.IDRef]
		if !ok || ref.Linear == "no" {
			continue
		}
		inSpine[it.ID] = true
		m.ReadingOrder = append(m.ReadingOrder, itemLink(base, it))
	}
	var navPath, ncxPath string
	for _, it := range opf.Items {
		link := itemLink(base, it)
		if strings.Contains(it.Properties, "cover-image") {
			link.Rel = []string{"cover"}
		}
		if strings.Contains(" "+it.Properties+" ", " nav ") {
			navPath = resolveHref(base, it.Href)
		}
		if it.ID == opf.Spine.TOC {
			ncxPath = resolveHref(base, it.Href)
		}
		if !inSpine[it.ID] {
			m.Resources = append(m.Resources, link)
		}
	}
	switch {
	case navPath != "":
		m.TOC = navTOC(zr, navPath)
	case ncxPath != "":
		m.TOC = ncxTOC(zr, ncxPath)
	}
	return m, nil
}

// buildWebPubPositions follows Readium's reflowable rule: one position per
// positionLength uncompressed bytes of each reading-order entry, at least one.
func buildWebPubPositions(zr *zip.Reader, m *WebPubManifest) *WebPubPositions {
	sizes := make(map[string]uint64, len(zr.File))
	for _, f := range zr.File {
		sizes[f.Name] = f.UncompressedSize64
	}
	counts := make([]int, len(m.ReadingOrder))
	total := 0
	for i, link := range m.ReadingOrder {
		size := min(sizes[strings.TrimPrefix(link.Href, resourceBaseDir)], maxWebPubEntry)
		counts[i] = max(1, int((size+positionLength-1)/positionLength))
		total += counts[i]
	}
	out := &WebPubPositions{Total: total, Positions: make([]WebPubLocator, 0, total)}
	for i, link := range m.ReadingOrder {
		for p := range counts[i] {
			n := len(out.Positions)
			out.Positions = append(out.Positions, WebPubLocator{
				Href: link.Href,
				Type: link.Type,
				Locations: WebPubLocations{
					Position:         n + 1,
					Progression:      float64(p) / float64(counts[i]),
					TotalProgression: float64(n) / float64(total),
				},
			})
		}
	}
	return out
}

func findOPFPath(zr *zip.Reader) (string, error) {
	var container struct {
		RootFiles []struct {
			FullPath  string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeZipXML(zr, "META-INF/container.xml", &container); err != nil {
		return "", err
	}
	for _, rf := range container.RootFiles {
		if rf.MediaType == opfMediaType {
			return rf.FullPath, nil
		}
	}
	return "", errors.New("container.xml names no package document")
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.TrimSpace(s[0])
}

// resolveHref maps a manifest href to a zip-root path.
func resolveHref(base, href string) string {
	if u, err := url.Parse(href); err == nil && u.Path != "" {
		href = u.Path
	}
	return path.Join(base, href)
}

func itemLink(base string, it opfItem) WebPubLink {
	return newLink(resourceBaseDir+resolveHref(base, it.Href), it.MediaType)
}

func newLink(href, mediaType string, rel ...string) WebPubLink {
	return WebPubLink{
		Href:       href,
		Type:       mediaType,
		Title:      "",
		Rel:        rel,
		Properties: nil,
		Children:   nil,
	}
}

func resourceType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".xhtml", ".html", ".htm":
		return mimeXHTML
	case ".opf":
		return opfMediaType
	case ".ncx":
		return "application/x-dtbncx+xml"
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// navTOC reads the EPUB3 nav document's toc <nav>.
func navTOC(zr *zip.Reader, navPath string) []WebPubLink {
	f, err := zr.Open(navPath)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	root, err := xhtml.Parse(f)
	if err != nil {
		return nil
	}
	var find func(*xhtml.Node) *xhtml.Node
	find = func(n *xhtml.Node) *xhtml.Node {
		if n.Type == xhtml.ElementNode && n.Data == "nav" {
			for _, a := range n.Attr {
				if strings.HasSuffix(a.Key, "type") && strings.Contains(a.Val, "toc") {
					return n
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if r := find(c); r != nil {
				return r
			}
		}
		return nil
	}
	nav := find(root)
	if nav == nil {
		return nil
	}
	return navList(nav, path.Dir(navPath))
}

func navList(n *xhtml.Node, dir string) []WebPubLink {
	var out []WebPubLink
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode {
			continue
		}
		switch c.Data {
		case "ol":
			out = append(out, navList(c, dir)...)
		case "li":
			out = append(out, navItem(c, dir))
		}
	}
	return out
}

func navItem(li *xhtml.Node, dir string) WebPubLink {
	link := newLink("", "")
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode {
			continue
		}
		switch c.Data {
		case "a", tagSpan:
			link.Title = strings.Join(strings.Fields(nodeText(c)), " ")
			for _, a := range c.Attr {
				if a.Key == "href" {
					link.Href = tocHref(dir, a.Val)
				}
			}
		case "ol":
			link.Children = navList(c, dir)
		}
	}
	return link
}

func nodeText(n *xhtml.Node) string {
	if n.Type == xhtml.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

// tocHref resolves a TOC href (keeping its #fragment) under res/.
func tocHref(dir, href string) string {
	if href == "" {
		return ""
	}
	p, frag, _ := strings.Cut(href, "#")
	out := resourceBaseDir + path.Join(dir, p)
	if p == "" {
		return ""
	}
	if frag != "" {
		out += "#" + frag
	}
	return out
}

type ncxPoint struct {
	Label struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []ncxPoint `xml:"navPoint"`
}

func ncxTOC(zr *zip.Reader, ncxPath string) []WebPubLink {
	var ncx struct {
		Points []ncxPoint `xml:"navMap>navPoint"`
	}
	if err := decodeZipXML(zr, ncxPath, &ncx); err != nil {
		return nil
	}
	return ncxLinks(ncx.Points, path.Dir(ncxPath))
}

func ncxLinks(points []ncxPoint, dir string) []WebPubLink {
	out := make([]WebPubLink, 0, len(points))
	for _, p := range points {
		link := newLink(tocHref(dir, p.Content.Src), "")
		link.Title = strings.TrimSpace(p.Label.Text)
		link.Children = ncxLinks(p.Children, dir)
		out = append(out, link)
	}
	return out
}
