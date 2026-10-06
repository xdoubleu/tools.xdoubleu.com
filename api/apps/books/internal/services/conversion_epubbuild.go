package services

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// imgTag is the <img> element tag name, shared by the EPUB-build
	// sanitization pass and the PDF-extract HTML rendering.
	imgTag = "img"
	// srcAttr is the img src attribute name, shared the same way.
	srcAttr = "src"

	contentTypeJPEG = "image/jpeg"
	contentTypePNG  = "image/png"
)

// imageMediaTypes maps image file extensions to their EPUB OPF manifest
// media-type.
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
// present alongside inPath are embedded automatically.
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
	article, err := buildArticleXHTML(htmlBytes, imgDir)
	if err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output epub: %w", err)
	}
	defer func() { _ = out.Close() }()

	if err = writeEPUBZip(out, meta, article, imgDir); err != nil {
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
	article articleXHTML,
	imgDir string,
) error {
	zw := zip.NewWriter(w)

	if err := writeStoredEntry(zw, "mimetype", "application/epub+zip"); err != nil {
		return err
	}
	images := article.images
	cover, hasCover := coverImage(meta)
	entries := []contentDoc{
		{Name: "META-INF/container.xml", XHTML: buildContainerXML()},
		{
			Name:  "OEBPS/content.opf",
			XHTML: buildContentOPF(meta, images, article.docs, cover, hasCover),
		},
		{
			Name:  "OEBPS/nav.xhtml",
			XHTML: buildNavXHTML(meta.Title, article.toc, article.pages),
		},
	}
	if hasCover {
		entries = append(entries, contentDoc{
			Name: "OEBPS/" + coverPage, XHTML: buildCoverXHTML(meta.Title, cover.FileName),
		})
		images = append([]epubImage{cover}, images...)
	}
	for _, doc := range article.docs {
		entries = append(entries, contentDoc{Name: "OEBPS/" + doc.Name, XHTML: doc.XHTML})
	}
	for _, e := range entries {
		if err := writeEntry(zw, e.Name, e.XHTML); err != nil {
			return err
		}
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

// escapeXMLText escapes the characters unsafe in XML text content (not
// attribute values, which this package never interpolates untrusted text
// into).
func escapeXMLText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
