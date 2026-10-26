package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func TestErrorCodes_FilesAndKobo(t *testing.T) {
	c := newBooksTestClient(t)
	unknown := uuid.NewString()

	runCodeCases(t, []codeCase{
		{"kobo sync unknown book", func(ctx context.Context) error {
			_, err := c.EnableKoboSync(
				ctx, &booksv1.EnableKoboSyncRequest{BookId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"finalize unknown upload", func(ctx context.Context) error {
			_, err := c.FinalizeBookUpload(ctx, &booksv1.FinalizeBookUploadRequest{
				UploadId: unknown, Filename: "x.epub",
			})
			return err
		}, connect.CodePermissionDenied},
		{"disconnect unknown device", func(ctx context.Context) error {
			_, err := c.DisconnectKoboDevice(
				ctx, &booksv1.DisconnectKoboDeviceRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"logs of unknown device", func(ctx context.Context) error {
			_, err := c.GetKoboDeviceLogs(
				ctx, &booksv1.GetKoboDeviceLogsRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"logging on unknown device", func(ctx context.Context) error {
			_, err := c.SetKoboDeviceLogging(
				ctx, &booksv1.SetKoboDeviceLoggingRequest{Id: unknown, Enabled: true},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid device id", func(ctx context.Context) error {
			_, err := c.ClearKoboDeviceLogs(
				ctx, &booksv1.ClearKoboDeviceLogsRequest{Id: "nope"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"catalog as non-admin", func(ctx context.Context) error {
			_, err := c.FindDuplicates(ctx, &booksv1.FindDuplicatesRequest{})
			return err
		}, connect.CodePermissionDenied},
	})
}

func TestErrorCodes_Catalog(t *testing.T) {
	c := newAdminBooksTestClient(t)
	unknown := uuid.NewString()

	runCodeCases(t, []codeCase{
		{"isbn of unknown book", func(ctx context.Context) error {
			_, err := c.SetBookISBN(ctx, &booksv1.SetBookISBNRequest{
				BookId: unknown, Isbn13: "9780306406157",
			})
			return err
		}, connect.CodeNotFound},
		{"update unknown book", func(ctx context.Context) error {
			_, err := c.UpdateBook(ctx, &booksv1.UpdateBookRequest{
				BookId: unknown, Metadata: &booksv1.Book{Title: "x"},
			})
			return err
		}, connect.CodeNotFound},
		{"sources of unknown book", func(ctx context.Context) error {
			_, err := c.GetBookSources(
				ctx, &booksv1.GetBookSourcesRequest{BookId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"apply source to unknown book", func(ctx context.Context) error {
			_, err := c.ApplyBookSource(ctx, &booksv1.ApplyBookSourceRequest{
				BookId: unknown, Source: "unicat",
			})
			return err
		}, connect.CodeNotFound},
		{"resync choice without proposal", func(ctx context.Context) error {
			_, err := c.ApplyResyncChoice(ctx, &booksv1.ApplyResyncChoiceRequest{
				BookId: unknown, Source: "unicat",
			})
			return err
		}, connect.CodeNotFound},
		{"unknown exact source", func(ctx context.Context) error {
			_, err := c.ListBooksInExactSources(
				ctx, &booksv1.ListBooksInExactSourcesRequest{Sources: []string{"x"}},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"invalid isbn", func(ctx context.Context) error {
			_, err := c.SetBookISBN(ctx, &booksv1.SetBookISBNRequest{
				BookId: unknown, Isbn13: "123",
			})
			return err
		}, connect.CodeInvalidArgument},
	})
}
