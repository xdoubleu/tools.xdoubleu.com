package services

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // registers GIF for image.DecodeConfig
	_ "image/jpeg" // registers JPEG for image.DecodeConfig
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"unicode"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"golang.org/x/net/html/charset"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// epubCharsPerPage approximates a printed page's characters, turning an
// EPUB's text length into pages.
const epubCharsPerPage = 2000

// FigureService measures the figure density of stored PDFs and EPUBs.
type FigureService struct {
	logger      *slog.Logger
	bookFiles   *repositories.BookFilesRepository
	objectStore objectstore.Client
}

func NewFigureService(
	logger *slog.Logger,
	bookFiles *repositories.BookFilesRepository,
	objectStore objectstore.Client,
) *FigureService {
	return &FigureService{
		logger:      logger,
		bookFiles:   bookFiles,
		objectStore: objectStore,
	}
}

// AnalysePending analyses up to limit unanalysed files, returning how many
// were recorded. A file that can't be fetched is retried on a later run; one
// that's missing or can't be parsed is recorded with zero pages, so it isn't
// retried.
func (s *FigureService) AnalysePending(ctx context.Context, limit int) (int, error) {
	if _, err := s.bookFiles.CopyFigureStats(ctx); err != nil {
		return 0, err
	}
	targets, err := s.bookFiles.ListUnanalysed(ctx, limit)
	if err != nil {
		return 0, err
	}

	recorded := 0
	for _, t := range targets {
		if err = ctx.Err(); err != nil {
			return recorded, err
		}
		stats, analyseErr := s.analyse(ctx, t)
		if analyseErr != nil {
			var fetchErr *figureFetchError
			if errors.As(analyseErr, &fetchErr) || ctx.Err() != nil {
				s.logger.WarnContext(ctx, "fetch book file for figure analysis",
					slog.String("key", t.StorageKey), slog.Any("error", analyseErr))
				continue
			}
			s.logger.WarnContext(ctx, "analyse book file figures",
				slog.String("key", t.StorageKey), slog.Any("error", analyseErr))
			stats = models.FigureStats{Figures: 0, Pages: 0}
		}
		if err = s.bookFiles.SetFigureStats(ctx, t.StorageKey, stats); err != nil {
			return recorded, err
		}
		recorded++
	}
	return recorded, nil
}

type figureFetchError struct{ err error }

func (e *figureFetchError) Error() string { return "fetch book file: " + e.err.Error() }

func (e *figureFetchError) Unwrap() error { return e.err }

func (s *FigureService) analyse(
	ctx context.Context,
	t models.FigureAnalysisTarget,
) (models.FigureStats, error) {
	var none models.FigureStats
	tmp, err := os.CreateTemp("", "figures-*")
	if err != nil {
		return none, &figureFetchError{err: err}
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	rc, err := s.objectStore.Get(ctx, t.StorageKey)
	if err != nil {
		exists, existsErr := s.objectStore.Exists(ctx, t.StorageKey)
		if existsErr == nil && !exists {
			return none, fmt.Errorf("book file missing: %w", err)
		}
		return none, &figureFetchError{err: err}
	}
	size, err := io.Copy(tmp, rc)
	_ = rc.Close()
	if err != nil {
		return none, &figureFetchError{err: err}
	}

	if t.Format == models.FileFormatPDF {
		return countPDFFigures(ctx, tmp.Name())
	}
	return countEPUBFigures(tmp, size)
}

// countPDFFigures counts the PDF's figures with the converter's raster and
// vector filters, over its page count.
func countPDFFigures(ctx context.Context, pdfPath string) (models.FigureStats, error) {
	var stats models.FigureStats
	err := withPDFDocument(ctx, pdfPath,
		func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error {
			countResp, err := instance.FPDF_GetPageCount(
				&requests.FPDF_GetPageCount{Document: doc},
			)
			if err != nil {
				return fmt.Errorf("get page count: %w", err)
			}
			for i := range countResp.PageCount {
				if err = ctx.Err(); err != nil {
					return err
				}
				page := requests.Page{ //nolint:exhaustruct // by index
					ByIndex: &requests.PageByIndex{Document: doc, Index: i},
				}
				n, pageErr := countPageFigures(instance, page)
				if pageErr != nil {
					return fmt.Errorf("page %d: %w", i, pageErr)
				}
				stats.Figures += n
			}
			stats.Pages = countResp.PageCount
			return nil
		})
	return stats, err
}

// countPageFigures counts the figures the converter would extract from the
// page, less a page-sized image under text (a scan's image layer).
func countPageFigures(instance pdfium.Pdfium, page requests.Page) (int, error) {
	geo, chars, err := pageText(instance, page)
	if err != nil {
		return 0, err
	}
	figures, _, err := pageFigures(instance, page, geo, chars)
	if err != nil {
		return 0, err
	}
	pageArea := geo.width * geo.height
	n := 0
	for _, f := range figures {
		if len(chars) > 0 &&
			(f.right-f.left)*(f.top-f.bottom) >= vectorMaxPageShare*pageArea {
			continue
		}
		n++
	}
	return n, nil
}

// withPDFDocument opens the PDF at path the way goPDFConverter does, holding
// pdfSem, and runs fn on it.
func withPDFDocument(
	ctx context.Context, path string,
	fn func(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) error,
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

	pdfBytes, err := os.ReadFile(path)
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

	return fn(instance, docResp.Document)
}

// countEPUBFigures counts the distinct figure-sized images (and inline SVG
// drawings) the spine shows, cover excluded, over pages estimated from the
// body text's length.
func countEPUBFigures(r io.ReaderAt, size int64) (models.FigureStats, error) {
	var none models.FigureStats
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return none, fmt.Errorf("open epub: %w", err)
	}
	opfPath, err := findOPFPath(zr)
	if err != nil {
		return none, err
	}
	var opf opfDoc
	if err = decodeZipXML(zr, opfPath, &opf); err != nil {
		return none, err
	}
	base := path.Dir(opfPath)

	items := make(map[string]opfItem, len(opf.Items))
	for _, it := range opf.Items {
		items[it.ID] = it
	}
	c := &epubFigureCounter{zr: zr, seen: map[string]bool{}, figures: 0, chars: 0}
	if cover, ok := opfCoverItem(opf); ok {
		c.seen[resolveHref(base, cover.Href)] = true
	}

	for _, ref := range opf.Spine.Refs {
		it, ok := items[ref.IDRef]
		if !ok {
			continue
		}
		if err = c.countDoc(resolveHref(base, it.Href)); err != nil {
			return none, err
		}
	}
	pages := max(1, (c.chars+epubCharsPerPage-1)/epubCharsPerPage)
	return models.FigureStats{Figures: c.figures, Pages: pages}, nil
}

type epubFigureCounter struct {
	zr      *zip.Reader
	seen    map[string]bool
	figures int
	chars   int
}

// countDoc adds a content document's body text length and figures: each
// <img>/<image>, and each inline <svg> drawing with no image inside.
func (c *epubFigureCounter) countDoc(name string) error {
	f, err := c.zr.Open(name)
	if err != nil {
		return nil // a missing spine document has nothing to count
	}
	defer func() { _ = f.Close() }()

	d := xml.NewDecoder(f)
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel

	doc := &epubDocScan{
		c:           c,
		dir:         path.Dir(name),
		inBody:      false,
		svgDepth:    0,
		svgHasImage: false,
	}
	for {
		tok, tokErr := d.Token()
		if errors.Is(tokErr, io.EOF) {
			return nil
		}
		if tokErr != nil {
			return fmt.Errorf("parse %s: %w", name, tokErr)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			doc.start(v)
		case xml.EndElement:
			doc.end(v)
		case xml.CharData:
			doc.text(v)
		}
	}
}

// epubDocScan is countDoc's position within one content document.
type epubDocScan struct {
	c           *epubFigureCounter
	dir         string
	inBody      bool
	svgDepth    int
	svgHasImage bool
}

func (s *epubDocScan) start(e xml.StartElement) {
	switch strings.ToLower(e.Name.Local) {
	case "body":
		s.inBody = true
	case "svg":
		if s.svgDepth == 0 {
			s.svgHasImage = false
		}
		s.svgDepth++
	case "img", "image":
		if !s.inBody {
			return
		}
		s.svgHasImage = s.svgHasImage || s.svgDepth > 0
		if src := imageSource(e); src != "" &&
			s.c.isFigureImage(resolveHref(s.dir, src)) {
			s.c.figures++
		}
	}
}

func (s *epubDocScan) end(e xml.EndElement) {
	if strings.ToLower(e.Name.Local) != "svg" || s.svgDepth == 0 {
		return
	}
	s.svgDepth--
	if s.svgDepth == 0 && s.inBody && !s.svgHasImage {
		s.c.figures++
	}
}

func (s *epubDocScan) text(data xml.CharData) {
	if !s.inBody {
		return
	}
	for _, r := range string(data) {
		if !unicode.IsSpace(r) {
			s.c.chars++
		}
	}
}

func imageSource(e xml.StartElement) string {
	for _, a := range e.Attr {
		if a.Name.Local == "src" || a.Name.Local == "href" {
			return a.Value
		}
	}
	return ""
}

// isFigureImage reports whether the zip image at name is a figure seen for
// the first time: an SVG, or a raster of at least figureMinPixels a side.
func (c *epubFigureCounter) isFigureImage(name string) bool {
	if c.seen[name] {
		return false
	}
	c.seen[name] = true
	if strings.EqualFold(path.Ext(name), ".svg") {
		return true
	}
	f, err := c.zr.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return false
	}
	return cfg.Width >= figureMinPixels && cfg.Height >= figureMinPixels
}
