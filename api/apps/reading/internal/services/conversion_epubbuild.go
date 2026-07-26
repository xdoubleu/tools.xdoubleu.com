package services

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// imageMediaTypes maps image file extensions to their EPUB OPF manifest
// media-type — the inverse of imageExtensions (ingest_images.go).
//
//nolint:gochecknoglobals // static lookup table
var imageMediaTypes = map[string]string{
	".jpg":  contentTypeJPEG,
	".png":  contentTypePNG,
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// epubImage is one embedded image discovered in the article body.
type epubImage struct {
	// FileName is the bare filename (e.g. "img_0.jpg"), used as both the
	// OEBPS/ zip entry name and the manifest href.
	FileName  string
	MediaType string
	// ID is the manifest item id, e.g. "item-img0".
	ID string
}

// goHTMLConverter builds a standalone EPUB 3 file at outPath from the HTML
// document at inPath, without shelling out to Calibre. Images already
// localized next to inPath by localizeImages are embedded automatically.
func goHTMLConverter(
	ctx context.Context, inPath, outPath string, meta ArticleMeta,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	htmlBytes, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read input html: %w", err)
	}

	imgDir := filepath.Dir(inPath)
	indexXHTML, images, err := buildArticleXHTML(htmlBytes, imgDir)
	if err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output epub: %w", err)
	}
	defer func() { _ = out.Close() }()

	if err = writeEPUBZip(out, meta, images, indexXHTML, imgDir); err != nil {
		return fmt.Errorf("write epub zip: %w", err)
	}
	return nil
}

// writeEPUBZip assembles the EPUB container: mimetype must be first and
// stored uncompressed, followed by the OPF package, nav document, article
// body, and any embedded images.
func writeEPUBZip(
	w io.Writer,
	meta ArticleMeta,
	images []epubImage,
	indexXHTML, imgDir string,
) error {
	zw := zip.NewWriter(w)

	if err := writeStoredEntry(zw, "mimetype", "application/epub+zip"); err != nil {
		return err
	}
	if err := writeEntry(
		zw, "META-INF/container.xml", buildContainerXML(),
	); err != nil {
		return err
	}
	if err := writeEntry(
		zw, "OEBPS/content.opf", buildContentOPF(meta, images),
	); err != nil {
		return err
	}
	if err := writeEntry(
		zw, "OEBPS/nav.xhtml", buildNavXHTML(meta.Title),
	); err != nil {
		return err
	}
	if err := writeEntry(zw, "OEBPS/index.xhtml", indexXHTML); err != nil {
		return err
	}
	for _, img := range images {
		if err := copyImageEntry(zw, imgDir, img); err != nil {
			return err
		}
	}

	return zw.Close()
}

// writeStoredEntry writes a zip entry with no compression — required for
// mimetype, which EPUB readers reject otherwise.
func writeStoredEntry(zw *zip.Writer, name, content string) error {
	//nolint:exhaustruct // only Name/Method matter for a stored entry
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   name,
		Method: zip.Store,
	})
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, err = io.WriteString(w, content)
	return err
}

func writeEntry(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, err = io.WriteString(w, content)
	return err
}

func copyImageEntry(zw *zip.Writer, imgDir string, img epubImage) error {
	f, err := os.Open(filepath.Join(imgDir, img.FileName))
	if err != nil {
		return fmt.Errorf("open image %s: %w", img.FileName, err)
	}
	defer func() { _ = f.Close() }()

	w, err := zw.Create("OEBPS/" + img.FileName)
	if err != nil {
		return fmt.Errorf("create zip entry for %s: %w", img.FileName, err)
	}
	_, err = io.Copy(w, f)
	return err
}

func buildContainerXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(
		`<container version="1.0" ` +
			`xmlns="urn:oasis:names:tc:opendocument:xmlns:container">` + "\n",
	)
	b.WriteString("  <rootfiles>\n")
	b.WriteString(
		`    <rootfile full-path="OEBPS/content.opf" ` +
			`media-type="application/oebps-package+xml"/>` + "\n",
	)
	b.WriteString("  </rootfiles>\n")
	b.WriteString("</container>\n")
	return b.String()
}

func buildContentOPF(meta ArticleMeta, images []epubImage) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" ` +
			`unique-identifier="pub-id" xml:lang="en">` + "\n",
	)
	b.WriteString(
		`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n",
	)
	b.WriteString(
		`    <dc:identifier id="pub-id">urn:uuid:` + uuid.NewString() +
			"</dc:identifier>\n",
	)
	b.WriteString(
		"    <dc:title>" + escapeXMLText(meta.Title) + "</dc:title>\n",
	)
	for _, author := range meta.Authors {
		b.WriteString(
			"    <dc:creator>" + escapeXMLText(author) + "</dc:creator>\n",
		)
	}
	b.WriteString("    <dc:language>en</dc:language>\n")
	b.WriteString("  </metadata>\n")

	b.WriteString("  <manifest>\n")
	b.WriteString(
		`    <item id="doc" href="index.xhtml" ` +
			`media-type="application/xhtml+xml"/>` + "\n",
	)
	b.WriteString(
		`    <item id="nav" href="nav.xhtml" ` +
			`media-type="application/xhtml+xml" properties="nav"/>` + "\n",
	)
	for _, img := range images {
		fmt.Fprintf(&b, "    <item id=\"%s\" href=\"%s\" media-type=\"%s\"/>\n",
			img.ID, img.FileName, img.MediaType)
	}
	b.WriteString("  </manifest>\n")

	b.WriteString("  <spine>\n")
	b.WriteString(`    <itemref idref="doc"/>` + "\n")
	b.WriteString("  </spine>\n")
	b.WriteString("</package>\n")

	return b.String()
}

func buildNavXHTML(title string) string {
	escaped := escapeXMLText(title)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(
		`<html xmlns="http://www.w3.org/1999/xhtml" ` +
			`xmlns:epub="http://www.idpf.org/2007/ops">` + "\n",
	)
	b.WriteString("<head><title>" + escaped + "</title></head>\n")
	b.WriteString("<body>\n")
	b.WriteString(`  <nav epub:type="toc" id="toc">` + "\n")
	b.WriteString("    <ol>\n")
	b.WriteString(
		`      <li><a href="index.xhtml">` + escaped + "</a></li>\n",
	)
	b.WriteString("    </ol>\n")
	b.WriteString("  </nav>\n")
	b.WriteString("</body>\n")
	b.WriteString("</html>\n")
	return b.String()
}

// escapeXMLText escapes the characters unsafe in XML text content (not
// attribute values, which this package never interpolates untrusted text
// into).
func escapeXMLText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
