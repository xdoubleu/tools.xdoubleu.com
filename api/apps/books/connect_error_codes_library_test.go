package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

type codeCase struct {
	name string
	call func(context.Context) error
	want connect.Code
}

func runCodeCases(t *testing.T, cases []codeCase) {
	t.Helper()
	ctx := testhelper.WithCookie(context.Background(), accessToken.String())
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(ctx)
			assert.Equal(t, tt.want, connect.CodeOf(err), "%v", err)
		})
	}
}

func TestErrorCodes_Library(t *testing.T) {
	c := newBooksTestClient(t)
	unknown := uuid.NewString()
	owned := addTestBook(t, "CodesBook").BookID.String()

	runCodeCases(t, []codeCase{
		{"status of unknown book", func(ctx context.Context) error {
			_, err := c.UpdateBookStatus(ctx, &booksv1.UpdateBookStatusRequest{
				BookId: unknown, Status: "reading",
			})
			return err
		}, connect.CodeNotFound},
		{"tag unknown book", func(ctx context.Context) error {
			_, err := c.SetBookTag(ctx, &booksv1.SetBookTagRequest{
				BookId: unknown, Tag: "poetry", Enabled: true,
			})
			return err
		}, connect.CodeNotFound},
		{"toggle tag of unknown book", func(ctx context.Context) error {
			_, err := c.ToggleTag(ctx, &booksv1.ToggleTagRequest{
				BookId: unknown, Tag: "poetry",
			})
			return err
		}, connect.CodeNotFound},
		{"content of unknown book", func(ctx context.Context) error {
			_, err := c.GetBookContent(
				ctx, &booksv1.GetBookContentRequest{BookId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"physical copy of unknown book", func(ctx context.Context) error {
			_, err := c.GetPhysicalCopy(
				ctx, &booksv1.GetPhysicalCopyRequest{BookId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid book id", func(ctx context.Context) error {
			_, err := c.RemoveBook(ctx, &booksv1.RemoveBookRequest{BookId: "nope"})
			return err
		}, connect.CodeInvalidArgument},
		{"invalid reading source", func(ctx context.Context) error {
			_, err := c.UpdateReadingProgress(
				ctx, &booksv1.UpdateReadingProgressRequest{
					BookId: owned, Source: "invalid-source", Percent: 10,
				},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"derived tag", func(ctx context.Context) error {
			_, err := c.SetBookTag(ctx, &booksv1.SetBookTagRequest{
				BookId: owned, Tag: "own-bol", Enabled: true,
			})
			return err
		}, connect.CodeInvalidArgument},
		{"create built-in shelf", func(ctx context.Context) error {
			_, err := c.CreateShelf(ctx, &booksv1.CreateShelfRequest{Name: "read"})
			return err
		}, connect.CodeInvalidArgument},
		{"delete built-in shelf", func(ctx context.Context) error {
			_, err := c.DeleteShelf(ctx, &booksv1.DeleteShelfRequest{
				Name: "read", TargetName: "to-read",
			})
			return err
		}, connect.CodeInvalidArgument},
		{"delete shelf into blank target", func(ctx context.Context) error {
			_, err := c.DeleteShelf(ctx, &booksv1.DeleteShelfRequest{
				Name: "codes-shelf", TargetName: "",
			})
			return err
		}, connect.CodeInvalidArgument},
		{"delete blank tag", func(ctx context.Context) error {
			_, err := c.DeleteTag(ctx, &booksv1.DeleteTagRequest{Name: ""})
			return err
		}, connect.CodeInvalidArgument},
		{"invalid finished date", func(ctx context.Context) error {
			_, err := c.UpdateFinishedAt(ctx, &booksv1.UpdateFinishedAtRequest{
				BookId: owned, FinishedAt: []string{"yesterday"},
			})
			return err
		}, connect.CodeInvalidArgument},
	})
}
