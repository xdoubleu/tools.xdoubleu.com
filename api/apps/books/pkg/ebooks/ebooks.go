// Package ebooks holds the EPUB/KEPUB format and book-file helpers shared by
// the services package and its conversion seam.
package ebooks

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html/charset"
)

// OPFMediaType is the package-document media type an EPUB's container.xml
// points at.
const OPFMediaType = "application/oebps-package+xml"

// BooksFolderPrefix is where every book's files live: books/<bookID>/<name>.
const BooksFolderPrefix = "books/"

// ExtKEPUB is the file extension of KEPUB files.
const ExtKEPUB = ".kepub"

// MaxUploadBytes caps one raw upload. Keep in sync with MAX_UPLOAD_BYTES in
// web/lib/backlog/zipFiles.ts.
const MaxUploadBytes = 250 * 1024 * 1024

// ArticleMeta carries the bibliographic fields passed to the HTML→EPUB
// converter (see conversion_epubbuild.go, shared by PDF-to-EPUB conversion).
// Identifier is the EPUB's unique-identifier (the OPF dc:identifier, usually
// "urn:uuid:<bookID>") — it must be stable across regenerations so the Kobo
// firmware can correlate a re-downloaded file with the book it already has.
type ArticleMeta struct {
	Title      string
	Authors    []string
	Identifier string
	// CoverImage is a cover image's filename beside the source HTML; the
	// EPUB then opens on a cover page showing it.
	CoverImage string
}

// BookFileKey returns books/<bookID>/<checksum><ext>.
func BookFileKey(bookID fmt.Stringer, checksum, ext string) string {
	return BooksFolderPrefix + bookID.String() + "/" + checksum + ext
}

// FindOPFPath returns the container.xml rootfile path whose media type is
// OPFMediaType.
func FindOPFPath(zr *zip.Reader) (string, error) {
	var container struct {
		RootFiles []struct {
			FullPath  string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := DecodeZipXML(zr, "META-INF/container.xml", &container); err != nil {
		return "", err
	}
	for _, rf := range container.RootFiles {
		if rf.MediaType == OPFMediaType {
			return rf.FullPath, nil
		}
	}
	return "", errors.New("container.xml names no package document")
}

// DecodeZipXML decodes one zip entry as XML, resolving payload-encoding
// charset labels.
func DecodeZipXML(zr *zip.Reader, name string, v any) error {
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

// OPFItem is one EPUB package-document manifest item.
type OPFItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

// OPFDoc is an EPUB package document: metadata, manifest and spine, with the
// parts the readers need.
type OPFDoc struct {
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Languages   []string `xml:"language"`
		Identifiers []string `xml:"identifier"`
		Metas       []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Items []OPFItem `xml:"manifest>item"`
	Spine struct {
		TOC  string `xml:"toc,attr"`
		Refs []struct {
			IDRef  string `xml:"idref,attr"`
			Linear string `xml:"linear,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
	Guide []struct {
		Type string `xml:"type,attr"`
		Href string `xml:"href,attr"`
	} `xml:"guide>reference"`
}

// OPFCoverItem returns the manifest item marked as the cover image.
func OPFCoverItem(opf OPFDoc) (OPFItem, bool) {
	for _, it := range opf.Items {
		if strings.Contains(" "+it.Properties+" ", " cover-image ") {
			return it, true
		}
	}
	for _, m := range opf.Metadata.Metas {
		if m.Name != "cover" {
			continue
		}
		for _, it := range opf.Items {
			if it.ID == m.Content && strings.HasPrefix(it.MediaType, "image/") {
				return it, true
			}
		}
	}
	return OPFItem{}, false //nolint:exhaustruct // zero value on a miss
}

// ResolveHref maps a manifest href to a zip-root path.
func ResolveHref(base, href string) string {
	if u, err := url.Parse(href); err == nil && u.Path != "" {
		href = u.Path
	}
	return path.Join(base, href)
}
