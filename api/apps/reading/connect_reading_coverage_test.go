package reading_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// --- SetBookCategory (connect_catalog.go) ---

func TestSetBookCategory_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   "00000000-0000-0000-0000-000000000001",
		Category: models.CategoryPaper,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestSetBookCategory_InvalidCategory_InvalidArgument(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetCategoryInvalidBook")
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   ub.BookID.String(),
		Category: "not-a-category",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetBookCategory_Success(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetCategorySuccessBook")
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   ub.BookID.String(),
		Category: models.CategoryPaper,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(
		context.Background(), ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryPaper, book.Category)
}

func TestAddBookByURL_RebuildsMissingFile(t *testing.T) {
	url := "https://blog.example.com/posts/" + uuid.NewString() + "/rebuild-me"
	mockWebFetch.SetHTML(url, articlePageHTML("Rebuild Me"))

	first, err := addByURL(t, url, "")
	require.NoError(t, err)
	bookID := mustUUID(t, first.UserBook.BookId)

	// Drop the stored EPUB so the re-add must rebuild it.
	_, err = testApp.Repositories.BookFiles.DeleteByUserBook(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)

	again, err := addByURL(t, url, "")
	require.NoError(t, err)
	assert.True(t, again.AlreadyInLibrary)

	status, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, status.HasEPUB, "missing file should be rebuilt on re-add")
}
