package conversion

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
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"golang.org/x/net/html/charset"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/ebooks"
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
	figures, err := figureBounds(instance, page, geo, chars)
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

// figureBounds returns the page's raster and vector figures as bounds-only
// rawFigures (png nil), mirroring the converter's extractPageImages and
// extractVectorFigures without their rendering. Figure counting never needs
// the pixels, and rendering them is what ballooned pdfium's wasm heap on a
// huge embedded image or page, OOM-killing the api (the restart loop).
func figureBounds(
	instance pdfium.Pdfium, page requests.Page, geo pageGeometry, chars []pdfChar,
) ([]rawFigure, error) {
	raster, err := imageFigureBounds(instance, page, geo.width*geo.height)
	if err != nil {
		return nil, fmt.Errorf("extract page images: %w", err)
	}
	vector, _, err := vectorFigureBounds(instance, page, geo, chars)
	if err != nil {
		return nil, fmt.Errorf("extract vector figures: %w", err)
	}
	return append(outsideFigures(raster, vector), vector...), nil
}

// imageFigureBounds enumerates a page's image objects that pass the
// converter's pixel-size and page-area filters, returning their bounds only.
func imageFigureBounds(
	instance pdfium.Pdfium, page requests.Page, pageArea float64,
) ([]rawFigure, error) {
	countResp, err := instance.FPDFPage_CountObjects(
		&requests.FPDFPage_CountObjects{Page: page},
	)
	if err != nil {
		return nil, fmt.Errorf("count page objects: %w", err)
	}

	var figures []rawFigure
	for i := range countResp.Count {
		objResp, getErr := instance.FPDFPage_GetObject(
			&requests.FPDFPage_GetObject{Page: page, Index: i},
		)
		if getErr != nil {
			return nil, fmt.Errorf("get page object %d: %w", i, getErr)
		}
		typeResp, typeErr := instance.FPDFPageObj_GetType(
			&requests.FPDFPageObj_GetType{PageObject: objResp.PageObject},
		)
		if typeErr != nil {
			return nil, fmt.Errorf("get page object type %d: %w", i, typeErr)
		}
		if typeResp.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}

		sizeResp, sizeErr := instance.FPDFImageObj_GetImagePixelSize(
			&requests.FPDFImageObj_GetImagePixelSize{ImageObject: objResp.PageObject},
		)
		if sizeErr != nil {
			return nil, fmt.Errorf("get image pixel size: %w", sizeErr)
		}
		if sizeResp.Width < figureMinPixels || sizeResp.Height < figureMinPixels {
			continue
		}

		boundsResp, boundsErr := instance.FPDFPageObj_GetBounds(
			&requests.FPDFPageObj_GetBounds{PageObject: objResp.PageObject},
		)
		if boundsErr != nil {
			return nil, fmt.Errorf("get image bounds: %w", boundsErr)
		}
		left, bottom := float64(boundsResp.Left), float64(boundsResp.Bottom)
		right, top := float64(boundsResp.Right), float64(boundsResp.Top)
		if pageArea <= 0 || (right-left)*(top-bottom) < figureMinAreaFraction*pageArea {
			continue
		}
		figures = append(figures, rawFigure{
			png: nil, left: left, top: top, right: right, bottom: bottom,
		})
	}
	return figures, nil
}

// vectorFigureBounds maps a page's vector figures to their region bounds, the
// converter's renderRegions crops without the page render.
func vectorFigureBounds(
	instance pdfium.Pdfium, page requests.Page, geo pageGeometry, chars []pdfChar,
) ([]rawFigure, []pdfChar, error) {
	boxes, err := pathBoxes(instance, page, geo)
	if err != nil || len(boxes) == 0 {
		return nil, chars, err
	}

	lines := groupLines(append([]pdfChar(nil), chars...))
	taken := make([]bool, len(lines))
	var regions []box
	for _, r := range clusterBoxes(boxes) {
		if r.width() < vectorMinSide || r.height() < vectorMinSide ||
			r.width()*r.height() < figureMinAreaFraction*geo.width*geo.height {
			continue
		}
		if region, ok := claimFigureRegion(r, lines, taken); ok {
			regions = append(regions, region)
		}
	}
	if len(regions) == 0 {
		return nil, chars, nil
	}
	regions = mergePanels(regions, lines, taken)

	figures := make([]rawFigure, 0, len(regions))
	for _, r := range regions {
		r = box{
			left: max(0, r.left-vectorPadding), bottom: max(0, r.bottom-vectorPadding),
			right: min(geo.width, r.right+vectorPadding),
			top:   min(geo.height, r.top+vectorPadding),
		}
		figures = append(figures, rawFigure{
			png: nil, left: r.left, top: r.top, right: r.right, bottom: r.bottom,
		})
	}
	return figures, charsOutsideLines(chars, lines, taken), nil
}

// withPDFDocument opens the PDF at path the way GoPDFConverter does, holding
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

	pool, err := getAnalysisPDFiumPool()
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
	opfPath, err := ebooks.FindOPFPath(zr)
	if err != nil {
		return none, err
	}
	var opf ebooks.OPFDoc
	if err = ebooks.DecodeZipXML(zr, opfPath, &opf); err != nil {
		return none, err
	}
	base := path.Dir(opfPath)

	items := make(map[string]ebooks.OPFItem, len(opf.Items))
	for _, it := range opf.Items {
		items[it.ID] = it
	}
	c := &epubFigureCounter{zr: zr, seen: map[string]bool{}, figures: 0, chars: 0}
	if cover, ok := ebooks.OPFCoverItem(opf); ok {
		c.seen[ebooks.ResolveHref(base, cover.Href)] = true
	}

	for _, ref := range opf.Spine.Refs {
		it, ok := items[ref.IDRef]
		if !ok {
			continue
		}
		if err = c.countDoc(ebooks.ResolveHref(base, it.Href)); err != nil {
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
			s.c.isFigureImage(ebooks.ResolveHref(s.dir, src)) {
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
