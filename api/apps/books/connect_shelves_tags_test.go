package books_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func TestConnectToggleTag_AddTag(t *testing.T) {
	book := addTestBook(t, "TagBook1")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "fantasy",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ToggleTag(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectToggleTag_RemoveTag(t *testing.T) {
	book := addTestBook(t, "TagBook2")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First add a tag
	addReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	})
	addReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, addReq)
	require.NoError(t, err)

	// Then remove it
	removeReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	})
	removeReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.ToggleTag(ctx, removeReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectToggleTag_EmptyTag(t *testing.T) {
	book := addTestBook(t, "TagBook3")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ToggleTag(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectCreateShelf_Success(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.CreateShelfRequest{Name: "empty-shelf"})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.CreateShelf(ctx, req)
	require.NoError(t, err)

	// The shelf must show up in the library with zero books, since nothing
	// was ever assigned to it.
	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)

	found := false
	for _, shelf := range libResp.Msg.Library.Shelves {
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

	req := connect.NewRequest(&booksv1.CreateShelfRequest{Name: models.StatusRead})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.CreateShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectCreateShelf_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.CreateShelfRequest{Name: ""})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.CreateShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestConnectShelf_PersistsWhenEmptied covers the core "shelves I'm lacking"
// fix: a custom shelf registered via UpdateBookStatus must keep showing up
// in GetLibrary even after its last book is moved off it.
func TestConnectShelf_PersistsWhenEmptied(t *testing.T) {
	book := addTestBook(t, "PersistShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "temporary-shelf",
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

	// Move the book back off the shelf.
	backReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusToRead,
	})
	backReq.Header().Set("Cookie", accessToken.String())
	_, err = client.UpdateBookStatus(ctx, backReq)
	require.NoError(t, err)

	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)

	found := false
	for _, shelf := range libResp.Msg.Library.Shelves {
		if shelf.Name == "temporary-shelf" {
			found = true
			assert.Empty(t, shelf.Books)
		}
	}
	assert.True(t, found, "temporary-shelf should persist after being emptied")
}

func TestConnectRenameShelf_Success(t *testing.T) {
	book := addTestBook(t, "RenameShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Give the book a custom shelf via UpdateBookStatus with a custom status.
	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "custom-shelf",
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

	// Rename the custom shelf.
	renameReq := connect.NewRequest(&booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: "renamed-shelf",
	})
	renameReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.RenameShelf(ctx, renameReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Moved, uint32(1))

	// Move the book off the renamed shelf: it must persist under its new
	// name (the registry entry moved with the rename), not the old one.
	backReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusToRead,
	})
	backReq.Header().Set("Cookie", accessToken.String())
	_, err = client.UpdateBookStatus(ctx, backReq)
	require.NoError(t, err)

	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)
	foundRenamed, foundOld := false, false
	for _, shelf := range libResp.Msg.Library.Shelves {
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

	req := connect.NewRequest(&booksv1.RenameShelfRequest{
		OldName: models.StatusToRead,
		NewName: "my-wishlist",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RenameShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectRenameShelf_TargetBuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: models.StatusToRead,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RenameShelf(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectRenameShelf_EmptyNewName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: "",
	})
	req.Header().Set("Cookie", accessToken.String())

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

	// Assign a custom shelf.
	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "shelf-to-delete",
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

	// Delete the shelf, moving books to to-read.
	deleteReq := connect.NewRequest(&booksv1.DeleteShelfRequest{
		Name:       "shelf-to-delete",
		TargetName: models.StatusToRead,
	})
	deleteReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.DeleteShelf(ctx, deleteReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Moved, uint32(1))

	// The deleted shelf must not reappear, even as an empty shelf.
	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)
	for _, shelf := range libResp.Msg.Library.Shelves {
		assert.NotEqual(t, "shelf-to-delete", shelf.Name)
	}
}

func TestConnectDeleteShelf_BuiltIn(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.DeleteShelfRequest{
		Name:       models.StatusReading,
		TargetName: models.StatusToRead,
	})
	req.Header().Set("Cookie", accessToken.String())

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

	// Add a tag first.
	tagReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "old-tag",
	})
	tagReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, tagReq)
	require.NoError(t, err)

	// Rename the tag.
	renameReq := connect.NewRequest(&booksv1.RenameTagRequest{
		OldName: "old-tag",
		NewName: "new-tag",
	})
	renameReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.RenameTag(ctx, renameReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Affected, uint32(1))
}

func TestConnectRenameTag_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.RenameTagRequest{
		OldName: "",
		NewName: "new-tag",
	})
	req.Header().Set("Cookie", accessToken.String())

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

	// Add a tag first.
	tagReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "tag-to-delete",
	})
	tagReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, tagReq)
	require.NoError(t, err)

	// Delete the tag.
	deleteReq := connect.NewRequest(&booksv1.DeleteTagRequest{
		Name: "tag-to-delete",
	})
	deleteReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.DeleteTag(ctx, deleteReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Affected, uint32(1))
}

func TestConnectDeleteTag_EmptyName(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.DeleteTagRequest{
		Name: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.DeleteTag(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
