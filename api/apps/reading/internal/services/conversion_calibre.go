package services

import (
	"context"
	"strings"
)

// calibreHTMLConverter shells out to ebook-convert (Calibre) to produce an
// EPUB from a standalone HTML document (images already localized next to it).
// It shares the single Calibre subprocess slot with PDF conversions.
func calibreHTMLConverter(
	ctx context.Context,
	inPath, outPath string,
	meta ArticleMeta,
) error {
	args := []string{"--no-default-epub-cover"}
	if meta.Title != "" {
		args = append(args, "--title", meta.Title)
	}
	if len(meta.Authors) > 0 {
		args = append(args, "--authors", strings.Join(meta.Authors, " & "))
	}
	return runEbookConvert(ctx, inPath, outPath, args...)
}
