package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// pdfSem allows one concurrent conversion: wazero keeps memory until an
// instance closes, so concurrency is a memory risk on a 512 MB box.
//
//nolint:gochecknoglobals // package-level semaphore; no per-instance state needed
var pdfSem = make(chan struct{}, 1)

// pdfiumPool is safe to reuse; the per-conversion instances borrowed from it
// must be closed right after use.
//
//nolint:gochecknoglobals // lazily-initialized singleton wasm runtime pool
var (
	pdfiumPoolOnce sync.Once
	pdfiumPool     pdfium.Pool
	//nolint:errname // cached init error, not a sentinel
	pdfiumPoolErr error
)

const pdfiumInstanceTimeout = 30 * time.Second

func getPDFiumPool() (pdfium.Pool, error) {
	pdfiumPoolOnce.Do(func() {
		pdfiumPool, pdfiumPoolErr = webassembly.Init(
			webassembly.Config{ //nolint:exhaustruct // zero-value defaults are correct
				MinIdle:  0,
				MaxIdle:  1,
				MaxTotal: 1,
			},
		)
	})
	return pdfiumPool, pdfiumPoolErr
}

// goPDFConverter converts a PDF to EPUB in pure Go: go-pdfium extracts
// text, positions and images, which are rebuilt as reading-order HTML for
// goHTMLConverter. Catalog title/authors win over PDF-derived metadata;
// identifier is stamped into dc:identifier.
func goPDFConverter(
	ctx context.Context, inPath, outPath, identifier, catalogTitle string,
	catalogAuthors []string,
) error {
	select {
	case pdfSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-pdfSem }()

	pool, err := getPDFiumPool()
	if err != nil {
		return fmt.Errorf("init pdfium pool: %w", err)
	}

	instance, err := pool.GetInstance(pdfiumInstanceTimeout)
	if err != nil {
		return fmt.Errorf("get pdfium instance: %w", err)
	}
	defer func() { _ = instance.Close() }()

	pdfBytes, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read pdf: %w", err)
	}

	docResp, err := instance.OpenDocument(
		&requests.OpenDocument{ //nolint:exhaustruct // only loading from bytes
			File: &pdfBytes,
		},
	)
	if err != nil {
		return fmt.Errorf("open pdf: %w", err)
	}
	defer func() {
		_, _ = instance.FPDF_CloseDocument(
			&requests.FPDF_CloseDocument{Document: docResp.Document},
		)
	}()

	workDir, err := os.MkdirTemp(filepath.Dir(outPath), "pdfextract-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	blocks, err := extractDocument(ctx, instance, docResp.Document, workDir)
	if err != nil {
		return fmt.Errorf("extract pdf content: %w", err)
	}

	meta := documentMeta(
		instance,
		docResp.Document,
		blocks,
		identifier,
		catalogTitle,
		catalogAuthors,
	)

	htmlPath := filepath.Join(workDir, "index.html")
	if err = os.WriteFile(htmlPath, []byte(renderHTML(blocks)), 0o600); err != nil {
		return fmt.Errorf("write extracted html: %w", err)
	}

	return goHTMLConverter(ctx, htmlPath, outPath, meta)
}

// renderHTML wraps the blocks in a minimal document for goHTMLConverter.
func renderHTML(blocks []htmlBlock) string {
	var b strings.Builder
	b.WriteString(
		"<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"/></head><body>\n",
	)
	for _, blk := range blocks {
		b.WriteString(blk.html)
		b.WriteByte('\n')
	}
	b.WriteString("</body></html>\n")
	return b.String()
}

// documentMeta prefers catalog title/author, then the PDF info dict, then the
// first heading (often frontmatter), then a placeholder.
func documentMeta(
	instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, blocks []htmlBlock,
	identifier, catalogTitle string, catalogAuthors []string,
) ArticleMeta {
	title := catalogTitle
	if title == "" {
		title = metaText(instance, doc, "Title")
	}
	if title == "" {
		title = firstHeadingText(blocks)
	}
	if title == "" {
		title = "Untitled"
	}

	authors := catalogAuthors
	if len(authors) == 0 {
		authors = splitAuthors(metaText(instance, doc, "Author"))
	}

	return ArticleMeta{
		Title:      title,
		Authors:    authors,
		Identifier: identifier,
	}
}

func metaText(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, tag string) string {
	resp, err := instance.FPDF_GetMetaText(
		&requests.FPDF_GetMetaText{Document: doc, Tag: tag},
	)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resp.Value)
}

func firstHeadingText(blocks []htmlBlock) string {
	for _, b := range blocks {
		if b.tag == "h1" || b.tag == "h2" {
			return b.text
		}
	}
	return ""
}

// splitAuthors splits a comma/semicolon-separated Author string.
func splitAuthors(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ',' })
	authors := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			authors = append(authors, p)
		}
	}
	return authors
}
