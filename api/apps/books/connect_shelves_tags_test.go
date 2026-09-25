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

	addReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	})
	addReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, addReq)
	require.NoError(t, err)

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

	// Registered shelves appear even with zero books.
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

// TestConnectShelf_PersistsWhenEmptied: a custom shelf survives losing its
// last book.
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

// TestConnectShelf_DroppedPersistsWhenEmptied: dropped is registered like a
// custom shelf.
func TestConnectShelf_DroppedPersistsWhenEmptied(t *testing.T) {
	book := addTestBook(t, "DroppedShelfBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: models.StatusDropped,
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

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

	libReq := connect.NewRequest(&booksv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	libResp, err := client.GetLibrary(ctx, libReq)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, shelf := range libResp.Msg.Library.Shelves {
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

	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "custom-shelf",
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

	renameReq := connect.NewRequest(&booksv1.RenameShelfRequest{
		OldName: "custom-shelf",
		NewName: "renamed-shelf",
	})
	renameReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.RenameShelf(ctx, renameReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Moved, uint32(1))

	// The registry entry moved with the rename.
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

	statusReq := connect.NewRequest(&booksv1.UpdateBookStatusRequest{
		BookId: book.BookID.String(),
		Status: "shelf-to-delete",
	})
	statusReq.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, statusReq)
	require.NoError(t, err)

	deleteReq := connect.NewRequest(&booksv1.DeleteShelfRequest{
		Name:       "shelf-to-delete",
		TargetName: models.StatusToRead,
	})
	deleteReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.DeleteShelf(ctx, deleteReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Msg.Moved, uint32(1))

	// The deleted shelf must not reappear, even empty.
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

	tagReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "old-tag",
	})
	tagReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, tagReq)
	require.NoError(t, err)

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

	tagReq := connect.NewRequest(&booksv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "tag-to-delete",
	})
	tagReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, tagReq)
	require.NoError(t, err)

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
