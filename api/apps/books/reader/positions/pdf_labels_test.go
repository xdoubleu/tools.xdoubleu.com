//nolint:testpackage // testing unexported page label mapping
package positions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
)

// labelledPDF is a PDF of pages blank pages whose catalog holds labels, a
// /PageLabels value ("" for none).
func labelledPDF(pages int, labels string) []byte {
	var buf bytes.Buffer
	offsets := []int{}
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.4\n")
	root := "<< /Type /Catalog /Pages 2 0 R"
	if labels != "" {
		root += " /PageLabels " + labels
	}
	obj(root + " >>")
	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", i+3)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", pages, kids))
	for range pages {
		obj("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] >>")
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets)+1, xref)
	return buf.Bytes()
}

func pdfPos(page int) models.ReadingPosition {
	return models.ReadingPosition{Href: "", Offset: 0, Page: page}
}

// romanThenArabic: pages 1-2 are i-ii, page 3 on is 1, 2, ...
const romanThenArabic = "<< /Nums [0 << /S /r >> 2 << /S /D >>] >>"

func TestReadPDFLabels_Mapping(t *testing.T) {
	data := labelledPDF(6, romanThenArabic)
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.True(t, l.usable())

	assert.Equal(t, 0, l.printedOf(1), "roman front matter has no printed page")
	assert.Equal(t, 0, l.printedOf(2))
	assert.Equal(t, 1, l.printedOf(3))
	assert.Equal(t, 4, l.printedOf(6))

	page, ok := l.pageOf(1)
	assert.True(t, ok)
	assert.Equal(t, 3, page, "printed 1 is PDF page 3")
	page, ok = l.pageOf(4)
	assert.True(t, ok)
	assert.Equal(t, 6, page)
	_, ok = l.pageOf(5)
	assert.False(t, ok, "past the last page")
}

func TestReadPDFLabels_StartPrefixAndKids(t *testing.T) {
	// Kids tree; arabic restarts at 10 on page 2 and a prefixed range follows.
	tree := "<< /Kids [<< /Nums [0 << /S /D /St 10 >> " +
		"3 << /S /D /P (A-) >>] >>] >>"
	data := labelledPDF(5, tree)
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.Equal(t, 10, l.printedOf(1))
	assert.Equal(t, 12, l.printedOf(3))
	assert.Equal(t, 0, l.printedOf(4), "a prefixed label isn't a printed page")
	page, ok := l.pageOf(11)
	assert.True(t, ok)
	assert.Equal(t, 2, page)
	_, ok = l.pageOf(13)
	assert.False(t, ok)
}

func TestReadPDFLabels_NoneOrBroken(t *testing.T) {
	data := labelledPDF(3, "")
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.False(t, l.usable())

	roman := labelledPDF(3, "<< /Nums [0 << /S /R >>] >>")
	l, err = readPDFLabels(bytes.NewReader(roman), int64(len(roman)))
	require.NoError(t, err)
	assert.False(t, l.usable(), "roman only")

	_, err = readPDFLabels(bytes.NewReader([]byte("nope")), 4)
	require.Error(t, err)
}

func newPDFFixture(t *testing.T, pdf []byte) *physicalFixture {
	t.Helper()
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), time.Minute)
	delete(f.files.byFormat, models.FileFormatEPUB)
	require.NoError(t, f.blobs.Put(context.Background(), "pdf", bytes.NewReader(pdf),
		int64(len(pdf)), "application/pdf"))
	pdfFile := &models.BookFile{} //nolint:exhaustruct // only these fields are read
	pdfFile.ID, pdfFile.Format = uuid.New(), models.FileFormatPDF
	pdfFile.StorageKey, pdfFile.Status = "pdf", models.FileStatusReady
	f.files.byFormat[models.FileFormatPDF] = pdfFile
	return f
}

func TestPhysicalService_PDFPrintedPageSync(t *testing.T) {
	f := newPDFFixture(t, labelledPDF(6, romanThenArabic))
	ctx := context.Background()
	id := uuid.New()

	require.NoError(t, f.svc.SyncPage(ctx, "u", id, 1, time.Now()))
	states := f.store.written()
	require.Len(t, states, 1)
	assert.Equal(t, &models.ReadingPosition{Href: "", Offset: 0, Page: 3},
		states[0].Position)
	assert.Equal(t, 50, states[0].Percent)

	require.NoError(t, f.svc.SyncPage(ctx, "u", id, 99, time.Now()))
	assert.Len(t, f.store.written(), 1, "a page the PDF lacks writes nothing")

	page, _ := f.svc.Locate(ctx, "u", id, pdfPos(5))
	assert.Equal(t, 3, page, "PDF page 5 is printed 3")
	page, _ = f.svc.Locate(ctx, "u", id, pdfPos(1))
	assert.Equal(t, 0, page)

	pos4 := pdfPos(4)
	f.svc.FollowDigital(ctx, models.BookReadingState{
		UserID: "u", BookID: id, Source: models.ReadingSourceWeb, Percent: 0,
		Location: nil, KoboLocation: nil, ReadAt: nil, UpdatedAt: time.Time{},
		Position: &pos4,
	}, time.Now())
	select {
	case got := <-f.store.pages:
		assert.Equal(t, 2, got)
	case <-time.After(5 * time.Second):
		t.Fatal("current page not followed")
	}

	view, err := f.svc.Copy(ctx, "u", id)
	require.NoError(t, err)
	assert.True(t, view.Syncable)
	assert.True(t, view.HasPrintPages)
}

func TestPhysicalService_PDFWithoutLabelsIsNotSyncable(t *testing.T) {
	f := newPDFFixture(t, labelledPDF(3, ""))
	ctx := context.Background()
	id := uuid.New()

	require.NoError(t, f.svc.SyncPage(ctx, "u", id, 1, time.Now()))
	assert.Empty(t, f.store.written())
	view, err := f.svc.Copy(ctx, "u", id)
	require.NoError(t, err)
	assert.False(t, view.HasPrintPages)
	page, _ := f.svc.Locate(ctx, "u", id, pdfPos(2))
	assert.Equal(t, 0, page)
}

func TestPositionService_PDFLabelsEdges(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	f := newPDFFixture(t, labelledPDF(6, romanThenArabic))
	pos := f.svc.positions
	pdfFile := f.files.byFormat[models.FileFormatPDF]

	require.NotNil(t, pos.pdfLabels(ctx, "u", id, time.Minute))
	getsBefore := f.blobs.gets.Load()
	require.NotNil(t, pos.pdfLabels(ctx, "u", id, time.Minute))
	assert.Equal(t, getsBefore, f.blobs.gets.Load(), "cached")

	pdfFile.Status = models.FileStatusConverting
	assert.Nil(t, pos.pdfLabels(ctx, "u", id, time.Minute), "not ready")
	pdfFile.Status = models.FileStatusReady

	f.files.err = errors.New("db down")
	assert.Nil(t, pos.pdfLabels(ctx, "u", id, time.Minute))
	f.files.err = nil

	pdfFile.ID, pdfFile.StorageKey = uuid.New(), "missing"
	assert.Nil(t, pos.pdfLabels(ctx, "u", id, time.Minute), "download fails, retried")

	pdfFile.ID, pdfFile.StorageKey = uuid.New(), "pdf"
	pos.maxDownload = 10
	assert.Nil(t, pos.pdfLabels(ctx, "u", id, time.Minute), "too large")
	_, cached := pos.labels.get(bookTextKey{fileID: pdfFile.ID, storageKey: "pdf"})
	assert.True(t, cached, "an oversized PDF is cached as unlabelled")
}
