package books_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

func TestConnectToggleTag_AddTag(t *testing.T) {
	book := addTestBook(t, "TagBook1")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "fantasy",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.ToggleTag(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
}

func TestConnectToggleTag_RemoveTag(t *testing.T) {
	book := addTestBook(t, "TagBook2")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addReq := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	}
	_, err := client.ToggleTag(testhelper.WithCookie(ctx, accessToken.String()), addReq)
	require.NoError(t, err)

	removeReq := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	}
	resp, err := client.ToggleTag(
		testhelper.WithCookie(ctx, accessToken.String()),
		removeReq,
	)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
}

func TestConnectToggleTag_EmptyTag(t *testing.T) {
	book := addTestBook(t, "TagBook3")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.ToggleTag(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectCreateShelf_Success(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.CreateShelfRequest{Name: "empty-shelf"}
	_, err := client.CreateShelf(testhelper.WithCookie(ctx, accessToken.String()), req)
	require.NoError(t, err)

	// Registered shelves appear even with zero books.
	libReq := &booksv1.GetLibraryRequest{}
	libResp, err := client.GetLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		libReq,
	)
	require.NoError(t, err)

	found := false
	for _, shelf := range libResp.Library.Shelves {
		if shelf.Name == "empty-shelf" {
			found = true
			assert.Empty(t, shelf.Books)
		}
	}
	assert.True(t, found, "empty-shelf should appear in the library shelves")
}

func TestConnectCreateShelf_BuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.CreateShelfRequest{Name: models.StatusRead}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.CreateShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectCreateShelf_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.CreateShelfRequest{Name: ""}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.CreateShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestConnectShelf_PersistsWhenEmptied: a custom shelf survives losing its
// last book.
func TestConnectShelf_PersistsWhenEmptied(t *testing.T) {
	book := addTestBook(t, "PersistShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "temporary-shelf",
	}
	_, err := client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		statusReq,
	)
	require.NoError(t, err)

	backReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusToRead,
	}
	_, err = client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		backReq,
	)
	require.NoError(t, err)

	libReq := &booksv1.GetLibraryRequest{}
	libResp, err := client.GetLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		libReq,
	)
	require.NoError(t, err)

	found := false
	for _, shelf := range libResp.Library.Shelves {
		if shelf.Name == "temporary-shelf" {
			found = true
			assert.Empty(t, shelf.Books)
		}
	}
	assert.True(t, found, "temporary-shelf should persist after being emptied")
}

// TestConnectShelf_DroppedPersistsWhenEmptied: dropped is registered like a
// custom shelf.
func TestConnectShelf_DroppedPersistsWhenEmptied(t *testing.T) {
	book := addTestBook(t, "DroppedShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusDropped,
	}
	_, err := client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		statusReq,
	)
	require.NoError(t, err)

	backReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusToRead,
	}
	_, err = client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		backReq,
	)
	require.NoError(t, err)

	libReq := &booksv1.GetLibraryRequest{}
	libResp, err := client.GetLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		libReq,
	)
	require.NoError(t, err)

	found := false
	for _, shelf := range libResp.Library.Shelves {
		if shelf.Name == models.StatusDropped {
			found = true
			assert.Empty(t, shelf.Books)
		}
	}
	assert.True(t, found, "dropped shelf should persist after being emptied")
}

// TestConnectLibrary_DroppedAlwaysPresent: dropped appears even with no
// registry or book history.
func TestConnectLibrary_DroppedAlwaysPresent(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	libReq := &booksv1.GetLibraryRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, shelf := range libResp.Library.Shelves {
		names[shelf.Name] = true
	}
	assert.True(
		t,
		names[models.StatusDropped],
		"dropped shelf should always be present",
	)
}

func TestConnectRenameShelf_Success(t *testing.T) {
	book := addTestBook(t, "RenameShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "custom-shelf",
	}
	_, err := client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		statusReq,
	)
	require.NoError(t, err)

	renameReq := &booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: "renamed-shelf",
	}
	resp, err := client.RenameShelf(
		testhelper.WithCookie(ctx, accessToken.String()),
		renameReq,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Moved, uint32(1))

	// The registry entry moved with the rename.
	backReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusToRead,
	}
	_, err = client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		backReq,
	)
	require.NoError(t, err)

	libReq := &booksv1.GetLibraryRequest{}
	libResp, err := client.GetLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		libReq,
	)
	require.NoError(t, err)
	foundRenamed, foundOld := false, false
	for _, shelf := range libResp.Library.Shelves {
		if shelf.Name == "renamed-shelf" {
			foundRenamed = true
		}
		if shelf.Name == "custom-shelf" {
			foundOld = true
		}
	}
	assert.True(t, foundRenamed, "renamed-shelf should persist after rename+empty")
	assert.False(t, foundOld, "custom-shelf should no longer exist after rename")
}

func TestConnectRenameShelf_BuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.RenameShelfRequest{
		OldName: models.StatusToRead,
		NewName: "my-wishlist",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RenameShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectRenameShelf_TargetBuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: models.StatusToRead,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RenameShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectRenameShelf_EmptyNewName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: "",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RenameShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectDeleteShelf_Success(t *testing.T) {
	book := addTestBook(t, "DeleteShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := &booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "shelf-to-delete",
	}
	_, err := client.UpdateBookStatus(
		testhelper.WithCookie(ctx, accessToken.String()),
		statusReq,
	)
	require.NoError(t, err)

	deleteReq := &booksv1.DeleteShelfRequest{
		Name:       "shelf-to-delete",
		TargetName: models.StatusToRead,
	}
	resp, err := client.DeleteShelf(
		testhelper.WithCookie(ctx, accessToken.String()),
		deleteReq,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Moved, uint32(1))

	// The deleted shelf must not reappear, even empty.
	libReq := &booksv1.GetLibraryRequest{}
	libResp, err := client.GetLibrary(
		testhelper.WithCookie(ctx, accessToken.String()),
		libReq,
	)
	require.NoError(t, err)
	for _, shelf := range libResp.Library.Shelves {
		assert.NotEqual(t, "shelf-to-delete", shelf.Name)
	}
}

func TestConnectDeleteShelf_BuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.DeleteShelfRequest{
		Name:       models.StatusReading,
		TargetName: models.StatusToRead,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.DeleteShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectRenameTag_Success(t *testing.T) {
	book := addTestBook(t, "RenameTagBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tagReq := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "old-tag",
	}
	_, err := client.ToggleTag(testhelper.WithCookie(ctx, accessToken.String()), tagReq)
	require.NoError(t, err)

	renameReq := &booksv1.RenameTagRequest{
		OldName: "old-tag",
		NewName: "new-tag",
	}
	resp, err := client.RenameTag(
		testhelper.WithCookie(ctx, accessToken.String()),
		renameReq,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Affected, uint32(1))
}

func TestConnectRenameTag_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.RenameTagRequest{
		OldName: "",
		NewName: "new-tag",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RenameTag(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectDeleteTag_Success(t *testing.T) {
	book := addTestBook(t, "DeleteTagBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tagReq := &booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "tag-to-delete",
	}
	_, err := client.ToggleTag(testhelper.WithCookie(ctx, accessToken.String()), tagReq)
	require.NoError(t, err)

	deleteReq := &booksv1.DeleteTagRequest{
		Name: "tag-to-delete",
	}
	resp, err := client.DeleteTag(
		testhelper.WithCookie(ctx, accessToken.String()),
		deleteReq,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Affected, uint32(1))
}

func TestConnectDeleteTag_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.DeleteTagRequest{
		Name: "",
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.DeleteTag(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
