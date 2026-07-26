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

// pdfSem limits PDF extraction to one concurrent conversion, mirroring the
// former calibreSem. Wazero does not return memory to the OS until an
// instance is closed, so concurrency here is a memory risk on a 512 MB box,
// not a throughput win.
//
//nolint:gochecknoglobals // package-level semaphore; no per-instance state needed
var pdfSem = make(chan struct{}, 1)

// pdfiumPool is a lazily-initialized, package-level wasm runtime pool. The
// pool itself (which compiles the embedded PDFium wasm module) is safe and
// cheap to reuse; per the operational constraints, it is per-conversion
// *instances* — borrowed from the pool and closed immediately after use —
// that must never be held long-lived.
//
//nolint:gochecknoglobals // lazily-initialized singleton wasm runtime pool
var (
	pdfiumPoolOnce sync.Once
	pdfiumPool     pdfium.Pool
	pdfiumPoolErr  error
)

const pdfiumInstanceTimeout = 30 * time.Second

func getPDFiumPool() (pdfium.Pool, error) {
	pdfiumPoolOnce.Do(func() {
		pdfiumPool, pdfiumPoolErr = webassembly.Init(webassembly.Config{
			MinIdle:  0,
			MaxIdle:  1,
			MaxTotal: 1,
		})
	})
	return pdfiumPool, pdfiumPoolErr
}

// goPDFConverter converts a PDF at inPath into an EPUB at outPath using a
// pure-Go pipeline: go-pdfium extracts text/positions/images (see
// conversion_pdfextract_page.go for the per-page/per-document orchestration),
// which are reassembled into reading-order HTML and handed to
// goHTMLConverter for EPUB assembly.
func goPDFConverter(ctx context.Context, inPath, outPath string) error {
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

	docResp, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
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

	meta := documentMeta(instance, docResp.Document, blocks)

	htmlPath := filepath.Join(workDir, "index.html")
	if err = os.WriteFile(htmlPath, []byte(renderHTML(blocks)), 0o600); err != nil {
		return fmt.Errorf("write extracted html: %w", err)
	}

	return goHTMLConverter(ctx, htmlPath, outPath, meta)
}

// renderHTML wraps the extracted blocks in a minimal standalone document —
// goHTMLConverter (and buildArticleXHTML underneath it) parses this the same
// way it parses a readability-extracted article body.
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

// documentMeta derives EPUB metadata from the PDF's own Title/Author info
// dictionary entries (PDFConverter has no metadata parameter — unlike
// HTMLConverter — since the source here is a raw PDF, not caller-supplied
// article metadata). Falls back to the document's first heading, then a
// fixed placeholder, when no Title is embedded.
func documentMeta(
	instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, blocks []htmlBlock,
) ArticleMeta {
	title := metaText(instance, doc, "Title")
	if title == "" {
		title = firstHeadingText(blocks)
	}
	if title == "" {
		title = "Untitled"
	}

	return ArticleMeta{
		Title:   title,
		Authors: splitAuthors(metaText(instance, doc, "Author")),
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

// splitAuthors splits a PDF's raw Author metadata string (typically
// comma/semicolon separated) into individual names.
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
