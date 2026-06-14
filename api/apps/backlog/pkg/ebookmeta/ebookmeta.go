package ebookmeta

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

const (
	FormatEPUB = "epub"
	FormatPDF  = "pdf"

	// magicPrefixLen is the number of bytes read for format detection.
	magicPrefixLen = 4

	// maxZipEntries caps the number of entries scanned in an EPUB zip.
	maxZipEntries = 10_000
	// maxXMLReadBytes caps decompressed bytes read from any single XML file
	// inside an EPUB (container.xml and the OPF) to guard against zip bombs.
	maxXMLReadBytes = 1 << 20 // 1 MB
)

// Metadata holds bibliographic data extracted from an ebook file.
type Metadata struct {
	Title    string
	Authors  []string
	ISBN13   *string
	ISBN10   *string
	Language *string
}

// DetectFormat returns FormatEPUB or FormatPDF based on magic bytes,
// filename extension, or MIME type (checked in that order).
// Returns empty string when the format cannot be determined.
func DetectFormat(magic []byte, filename, contentType string) string {
	if len(magic) >= magicPrefixLen {
		// PK\x03\x04 = zip/EPUB
		if magic[0] == 0x50 && magic[1] == 0x4B &&
			magic[2] == 0x03 && magic[3] == 0x04 {
			return FormatEPUB
		}
		// %PDF
		if bytes.Equal(magic[:magicPrefixLen], []byte("%PDF")) {
			return FormatPDF
		}
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".epub"):
		return FormatEPUB
	case strings.HasSuffix(lower, ".pdf"):
		return FormatPDF
	}
	switch contentType {
	case "application/epub+zip":
		return FormatEPUB
	case "application/pdf":
		return FormatPDF
	}
	return ""
}

// DetectFormatFromMagic returns FormatEPUB or FormatPDF based solely on the
// first four magic bytes of data. Returns empty string when neither matches.
// Use this on the server path — never trust filename or content-type alone.
func DetectFormatFromMagic(data []byte) string {
	if len(data) < magicPrefixLen {
		return ""
	}
	if data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04 {
		return FormatEPUB
	}
	if bytes.Equal(data[:magicPrefixLen], []byte("%PDF")) {
		return FormatPDF
	}
	return ""
}

// Extract reads bibliographic metadata from r.
// format must be FormatEPUB or FormatPDF.
func Extract(
	format string,
	r io.ReaderAt,
	size int64,
) (Metadata, error) {
	switch format {
	case FormatEPUB:
		return extractEPUB(r, size)
	case FormatPDF:
		return extractPDF(r, size)
	default:
		return Metadata{}, fmt.Errorf("ebookmeta: unsupported format %q", format)
	}
}

// --- EPUB ---

type epubContainer struct {
	Rootfiles []epubRootfile `xml:"rootfiles>rootfile"`
}

type epubRootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

type opfPackage struct {
	Metadata opfMetadata `xml:"http://www.idpf.org/2007/opf metadata"`
}

type opfMetadata struct {
	Titles []string `xml:"http://purl.org/dc/elements/1.1/ title"`

	Creators    []string        `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Identifiers []opfIdentifier `xml:"http://purl.org/dc/elements/1.1/ identifier"`
	Languages   []string        `xml:"http://purl.org/dc/elements/1.1/ language"`
}

type opfIdentifier struct {
	Value  string `xml:",chardata"`
	Scheme string `xml:"http://www.idpf.org/2007/opf scheme,attr"`
}

func extractEPUB(r io.ReaderAt, size int64) (Metadata, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return Metadata{}, fmt.Errorf("ebookmeta: open epub zip: %w", err)
	}
	if len(zr.File) > maxZipEntries {
		return Metadata{}, fmt.Errorf(
			"ebookmeta: zip has too many entries (%d)",
			len(zr.File),
		)
	}
	opfPath, err := epubOPFPath(zr)
	if err != nil {
		return Metadata{}, err
	}
	f := zipFile(zr, opfPath)
	if f == nil {
		return Metadata{}, fmt.Errorf("ebookmeta: OPF not found at %q", opfPath)
	}
	rc, err := f.Open()
	if err != nil {
		return Metadata{}, fmt.Errorf("ebookmeta: open OPF: %w", err)
	}
	defer rc.Close()

	var pkg opfPackage
	decodeErr := xml.NewDecoder(io.LimitReader(rc, maxXMLReadBytes)).Decode(&pkg)
	if decodeErr != nil {
		return Metadata{}, fmt.Errorf("ebookmeta: parse OPF: %w", decodeErr)
	}
	return opfToMetadata(pkg.Metadata), nil
}

func opfToMetadata(meta opfMetadata) Metadata {
	var m Metadata
	if len(meta.Titles) > 0 {
		m.Title = strings.TrimSpace(meta.Titles[0])
	}
	for _, c := range meta.Creators {
		if s := strings.TrimSpace(c); s != "" {
			m.Authors = append(m.Authors, s)
		}
	}
	if len(meta.Languages) > 0 {
		if lang := strings.TrimSpace(meta.Languages[0]); lang != "" {
			m.Language = &lang
		}
	}
	for _, id := range meta.Identifiers {
		i13, i10 := classifyISBN(id.Scheme, id.Value)
		if i13 != nil && m.ISBN13 == nil {
			m.ISBN13 = i13
		}
		if i10 != nil && m.ISBN10 == nil {
			m.ISBN10 = i10
		}
	}
	return m
}

func epubOPFPath(zr *zip.Reader) (string, error) {
	cf := zipFile(zr, "META-INF/container.xml")
	if cf == nil {
		return "", fmt.Errorf(
			"ebookmeta: META-INF/container.xml not found in epub",
		)
	}
	rc, err := cf.Open()
	if err != nil {
		return "", fmt.Errorf("ebookmeta: open container.xml: %w", err)
	}
	defer rc.Close()

	var c epubContainer
	decodeErr := xml.NewDecoder(io.LimitReader(rc, maxXMLReadBytes)).Decode(&c)
	if decodeErr != nil {
		return "", fmt.Errorf("ebookmeta: parse container.xml: %w", decodeErr)
	}
	for _, rf := range c.Rootfiles {
		if rf.MediaType == "application/oebps-package+xml" {
			return rf.FullPath, nil
		}
	}
	if len(c.Rootfiles) > 0 {
		return c.Rootfiles[0].FullPath, nil
	}
	return "", fmt.Errorf("ebookmeta: no rootfile in container.xml")
}

func zipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// --- PDF ---

func extractPDF(r io.ReaderAt, size int64) (Metadata, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return Metadata{}, fmt.Errorf("ebookmeta: open pdf: %w", err)
	}
	info := reader.Trailer().Key("Info")
	var m Metadata
	if title := info.Key("Title").Text(); title != "" {
		m.Title = title
	}
	if author := info.Key("Author").Text(); author != "" {
		m.Authors = []string{author}
	}
	return m, nil
}

// --- ISBN detection ---

var nonISBNRe = regexp.MustCompile(`[^\dX]`)

// classifyISBN returns (*isbn13, *isbn10) — at most one non-nil — detected from value.
func classifyISBN(_ string, value string) (*string, *string) {
	clean := nonISBNRe.ReplaceAllString(
		strings.ToUpper(strings.TrimSpace(value)), "",
	)
	switch {
	case len(clean) == 13 && allDigits(clean) &&
		(strings.HasPrefix(clean, "978") || strings.HasPrefix(clean, "979")):
		s := clean
		return &s, nil
	case len(clean) == 10 &&
		allDigits(clean[:9]) &&
		(clean[9] >= '0' && clean[9] <= '9' || clean[9] == 'X'):
		s := clean
		return nil, &s
	}
	return nil, nil
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
