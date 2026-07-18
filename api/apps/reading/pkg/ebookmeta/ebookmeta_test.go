package ebookmeta_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/pkg/ebookmeta"
)

// ---- fixture builders ----

func buildTestEPUB(
	title string,
	authors []string,
	isbn string,
	language string,
) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	const rootfileLine = `<rootfile full-path="OEBPS/content.opf"` +
		` media-type="application/oebps-package+xml"/>`

	writeZipEntry(zw, "META-INF/container.xml",
		`<?xml version="1.0"?>`+
			`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"`+
			` version="1.0">`+
			`<rootfiles>`+rootfileLine+`</rootfiles>`+
			`</container>`,
	)

	var opf strings.Builder
	opf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	opf.WriteString(
		`<package xmlns="http://www.idpf.org/2007/opf"` +
			` xmlns:dc="http://purl.org/dc/elements/1.1/"` +
			` xmlns:opf="http://www.idpf.org/2007/opf"` +
			` version="2.0">`,
	)
	opf.WriteString(`<metadata>`)
	fmt.Fprintf(&opf, `<dc:title>%s</dc:title>`, escapeXML(title))
	for _, a := range authors {
		fmt.Fprintf(&opf, `<dc:creator>%s</dc:creator>`, escapeXML(a))
	}
	if isbn != "" {
		fmt.Fprintf(
			&opf,
			`<dc:identifier opf:scheme="ISBN">%s</dc:identifier>`,
			isbn,
		)
	}
	if language != "" {
		fmt.Fprintf(&opf, `<dc:language>%s</dc:language>`, escapeXML(language))
	}
	opf.WriteString(`</metadata><manifest/><spine toc="ncx"/></package>`)

	writeZipEntry(zw, "OEBPS/content.opf", opf.String())
	_ = zw.Close()
	return buf.Bytes()
}

func writeZipEntry(zw *zip.Writer, name, content string) {
	f, _ := zw.Create(name)
	_, _ = f.Write([]byte(content))
}

func escapeXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// buildTestPDF creates a minimal valid PDF with Info metadata.
func buildTestPDF(title, author string) []byte {
	var b bytes.Buffer

	b.WriteString("%PDF-1.4\n")

	off1 := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	off2 := b.Len()
	b.WriteString(
		"2 0 obj\n<< /Type /Pages /Kids [ 3 0 R ] /Count 1 >>\nendobj\n",
	)

	off3 := b.Len()
	b.WriteString(
		"3 0 obj\n<< /Type /Page /Parent 2 0 R" +
			" /MediaBox [ 0 0 612 792 ] >>\nendobj\n",
	)

	off4 := b.Len()
	fmt.Fprintf(
		&b,
		"4 0 obj\n<< /Title (%s) /Author (%s) >>\nendobj\n",
		title,
		author,
	)

	xrefOff := b.Len()
	b.WriteString("xref\n0 5\n")
	b.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", off1)
	fmt.Fprintf(&b, "%010d 00000 n \n", off2)
	fmt.Fprintf(&b, "%010d 00000 n \n", off3)
	fmt.Fprintf(&b, "%010d 00000 n \n", off4)
	fmt.Fprintf(
		&b,
		"trailer\n<< /Size 5 /Root 1 0 R /Info 4 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		xrefOff,
	)

	return b.Bytes()
}

// ---- DetectFormat ----

func TestDetectFormat(t *testing.T) {
	t.Parallel()

	epubMagic := []byte{0x50, 0x4B, 0x03, 0x04}
	pdfMagic := []byte("%PDF-1.4 rest")

	tests := []struct {
		name        string
		magic       []byte
		filename    string
		contentType string
		want        string
	}{
		{
			name:        "epub magic bytes",
			magic:       epubMagic,
			filename:    "",
			contentType: "",
			want:        ebookmeta.FormatEPUB,
		},
		{
			name:        "pdf magic bytes",
			magic:       pdfMagic,
			filename:    "",
			contentType: "",
			want:        ebookmeta.FormatPDF,
		},
		{
			name:        "epub filename fallback",
			magic:       []byte{0x00, 0x01},
			filename:    "book.EPUB",
			contentType: "",
			want:        ebookmeta.FormatEPUB,
		},
		{
			name:        "pdf filename fallback",
			magic:       []byte{0x00, 0x01},
			filename:    "document.pdf",
			contentType: "",
			want:        ebookmeta.FormatPDF,
		},
		{
			name:        "epub content-type fallback",
			magic:       nil,
			filename:    "",
			contentType: "application/epub+zip",
			want:        ebookmeta.FormatEPUB,
		},
		{
			name:        "pdf content-type fallback",
			magic:       nil,
			filename:    "",
			contentType: "application/pdf",
			want:        ebookmeta.FormatPDF,
		},
		{
			name:        "unknown returns empty",
			magic:       []byte{0xDE, 0xAD},
			filename:    "",
			contentType: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ebookmeta.DetectFormat(tt.magic, tt.filename, tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---- Extract EPUB ----

func ptr(s string) *string { return &s }

func TestExtract_EPUB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		title    string
		authors  []string
		isbn     string
		language string
		want     ebookmeta.Metadata
	}{
		{
			name:     "single author no isbn",
			title:    "Go Programming",
			authors:  []string{"Alan Donovan"},
			isbn:     "",
			language: "",
			want: ebookmeta.Metadata{
				Title:    "Go Programming",
				Authors:  []string{"Alan Donovan"},
				ISBN13:   nil,
				Language: nil,
			},
		},
		{
			name:     "multiple authors",
			title:    "The Pragmatic Programmer",
			authors:  []string{"David Thomas", "Andrew Hunt"},
			isbn:     "",
			language: "",
			want: ebookmeta.Metadata{
				Title:    "The Pragmatic Programmer",
				Authors:  []string{"David Thomas", "Andrew Hunt"},
				ISBN13:   nil,
				Language: nil,
			},
		},
		{
			name:     "isbn13 with hyphens",
			title:    "Clean Code",
			authors:  []string{"Robert Martin"},
			isbn:     "978-0-13-235088-4",
			language: "en",
			want: ebookmeta.Metadata{
				Title:    "Clean Code",
				Authors:  []string{"Robert Martin"},
				ISBN13:   ptr("9780132350884"),
				Language: ptr("en"),
			},
		},
		{
			name:     "isbn10 not stored",
			title:    "SICP",
			authors:  []string{"Harold Abelson"},
			isbn:     "0-262-51087-1",
			language: "",
			want: ebookmeta.Metadata{
				Title:    "SICP",
				Authors:  []string{"Harold Abelson"},
				ISBN13:   nil,
				Language: nil,
			},
		},
		{
			name:     "language only",
			title:    "Das Schloss",
			authors:  []string{"Franz Kafka"},
			isbn:     "",
			language: "de",
			want: ebookmeta.Metadata{
				Title:    "Das Schloss",
				Authors:  []string{"Franz Kafka"},
				ISBN13:   nil,
				Language: ptr("de"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := buildTestEPUB(tt.title, tt.authors, tt.isbn, tt.language)
			r := bytes.NewReader(data)

			got, err := ebookmeta.Extract(
				ebookmeta.FormatEPUB,
				r,
				int64(len(data)),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want.Title, got.Title)
			assert.Equal(t, tt.want.Authors, got.Authors)
			assert.Equal(t, tt.want.ISBN13, got.ISBN13)
			assert.Equal(t, tt.want.Language, got.Language)
		})
	}
}

func TestExtract_EPUB_InvalidZip(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("not a zip file at all")
	_, err := ebookmeta.Extract(
		ebookmeta.FormatEPUB,
		strings.NewReader("not a zip"),
		int64(r.Len()),
	)
	assert.Error(t, err)
}

func TestExtract_EPUB_TooManyEntries(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// maxZipEntries is 10_000 — exceed it to trigger the guard.
	for i := range 10_001 {
		f, _ := zw.Create(fmt.Sprintf("entry%d.txt", i))
		_, _ = f.Write(nil)
	}
	_ = zw.Close()

	data := buf.Bytes()
	_, err := ebookmeta.Extract(
		ebookmeta.FormatEPUB,
		bytes.NewReader(data),
		int64(len(data)),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many entries")
}

func TestDetectFormatFromMagic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"epub magic", []byte{0x50, 0x4B, 0x03, 0x04, 0x00}, ebookmeta.FormatEPUB},
		{"pdf magic", []byte("%PDF-rest"), ebookmeta.FormatPDF},
		{"wrong magic", []byte{0x00, 0x01, 0x02, 0x03}, ""},
		{"too short", []byte{0x50, 0x4B}, ""},
		{"empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ebookmeta.DetectFormatFromMagic(tt.data))
		})
	}
}

// ---- Extract PDF ----

func TestExtract_PDF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		title  string
		author string
		want   ebookmeta.Metadata
	}{
		{
			name:   "title and author",
			title:  "Structure and Interpretation",
			author: "Abelson",
			want: ebookmeta.Metadata{
				Title:    "Structure and Interpretation",
				Authors:  []string{"Abelson"},
				ISBN13:   nil,
				Language: nil,
			},
		},
		{
			name:   "empty title and author",
			title:  "",
			author: "",
			want: ebookmeta.Metadata{
				Title:    "",
				Authors:  nil,
				ISBN13:   nil,
				Language: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := buildTestPDF(tt.title, tt.author)
			r := bytes.NewReader(data)

			got, err := ebookmeta.Extract(
				ebookmeta.FormatPDF,
				r,
				int64(len(data)),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want.Title, got.Title)
			assert.Equal(t, tt.want.Authors, got.Authors)
		})
	}
}

// ---- Extract unsupported format ----

func TestExtract_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("")
	_, err := ebookmeta.Extract("mobi", r, 0)
	assert.ErrorContains(t, err, "unsupported format")
}
