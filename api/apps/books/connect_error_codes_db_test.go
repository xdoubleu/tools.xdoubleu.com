package books_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// newBrokenDBClient serves books over a DB where every books-schema
// statement fails.
func newBrokenDBClient(t *testing.T, authService auth.Service) booksTestClient {
	t.Helper()
	app := books.NewInner(
		context.Background(),
		authService,
		logging.NewNopLogger(),
		testCfg,
		failingDB{DB: testDB, failSQL: "books."},
		books.Clients{
			UniCat:           nil,
			Hardcover:        mocks.NewMockHardcoverClient(),
			ObjectStore:      objectstore.NewFake(),
			WebFetch:         nil,
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connecthttp.WithHTTPGet())
}

func TestErrorCodes_DBFailureIsInternal(t *testing.T) {
	c := newBrokenDBClient(t, sharedmocks.NewMockedAuthService(userID))
	admin := newBrokenDBClient(t, sharedmocks.NewMockedAdminAuthService(userID))
	id := uuid.NewString()

	runCodeCases(t, []codeCase{
		{"library", func(ctx context.Context) error {
			_, err := c.GetLibrary(ctx, &booksv1.GetLibraryRequest{})
			return err
		}, connect.CodeInternal},
		{"progress", func(ctx context.Context) error {
			_, err := c.GetBooksProgress(ctx, &booksv1.GetBooksProgressRequest{})
			return err
		}, connect.CodeInternal},
		{"search", func(ctx context.Context) error {
			_, err := c.SearchLibrary(ctx, &booksv1.SearchLibraryRequest{Query: "x"})
			return err
		}, connect.CodeInternal},
		{"create", func(ctx context.Context) error {
			_, err := c.CreateBook(ctx, &booksv1.CreateBookRequest{
				Provider: "manual", Title: "x", Author: "y",
			})
			return err
		}, connect.CodeInternal},
		{"status", func(ctx context.Context) error {
			_, err := c.UpdateBookStatus(ctx, &booksv1.UpdateBookStatusRequest{
				BookId: id, Status: "reading",
			})
			return err
		}, connect.CodeInternal},
		{"finished dates", func(ctx context.Context) error {
			_, err := c.UpdateFinishedAt(
				ctx, &booksv1.UpdateFinishedAtRequest{BookId: id},
			)
			return err
		}, connect.CodeInternal},
		{"remove", func(ctx context.Context) error {
			_, err := c.RemoveBook(ctx, &booksv1.RemoveBookRequest{BookId: id})
			return err
		}, connect.CodeInternal},
		{"reading state", func(ctx context.Context) error {
			_, err := c.GetReadingState(
				ctx, &booksv1.GetReadingStateRequest{BookId: id},
			)
			return err
		}, connect.CodeInternal},
		{"series", func(ctx context.Context) error {
			_, err := c.GetSeries(ctx, &booksv1.GetSeriesRequest{Name: "x"})
			return err
		}, connect.CodeInternal},
		{"kepub status", func(ctx context.Context) error {
			_, err := c.GetKEPUBStatus(
				ctx, &booksv1.GetKEPUBStatusRequest{BookId: id},
			)
			return err
		}, connect.CodeInternal},
		{"convert", func(ctx context.Context) error {
			_, err := c.RequestKEPUBConversion(
				ctx, &booksv1.RequestKEPUBConversionRequest{BookId: id},
			)
			return err
		}, connect.CodeInternal},
		{"register device", func(ctx context.Context) error {
			_, err := c.RegisterKoboDevice(ctx, &booksv1.RegisterKoboDeviceRequest{
				Name: "x", Serial: "y",
			})
			return err
		}, connect.CodeInternal},
		{"devices", func(ctx context.Context) error {
			_, err := c.ListKoboDevices(ctx, &booksv1.ListKoboDevicesRequest{})
			return err
		}, connect.CodeInternal},
		{"duplicates", func(ctx context.Context) error {
			_, err := admin.FindDuplicates(ctx, &booksv1.FindDuplicatesRequest{})
			return err
		}, connect.CodeInternal},
		{"merge", func(ctx context.Context) error {
			_, err := admin.MergeBooks(ctx, &booksv1.MergeBooksRequest{
				WinnerBookId: id, LoserBookIds: []string{uuid.NewString()},
			})
			return err
		}, connect.CodeInternal},
		{"resync proposals", func(ctx context.Context) error {
			_, err := admin.ListResyncProposals(
				ctx, &booksv1.ListResyncProposalsRequest{},
			)
			return err
		}, connect.CodeInternal},
		{"source stats", func(ctx context.Context) error {
			_, err := admin.GetSourceStats(ctx, &booksv1.GetSourceStatsRequest{})
			return err
		}, connect.CodeInternal},
	})
}
