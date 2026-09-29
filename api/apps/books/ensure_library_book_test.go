package books_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
)

func learnProposal(title string) services.SourceProposal {
	//nolint:exhaustruct //optional fields not needed
	return services.SourceProposal{
		Source:  "manual",
		Title:   title,
		Authors: []string{"Test Author"},
		ISBN13:  testISBN(title),
	}
}

func TestAddToLibraryIfAbsent_AddsToShelf(t *testing.T) {
	ub, err := testApp.Services.Books.AddToLibraryIfAbsent(
		context.Background(), userID, learnProposal("IfAbsent New"), "To Learn",
	)
	require.NoError(t, err)
	assert.Equal(t, "To Learn", ub.Status)
}

func TestAddToLibraryIfAbsent_OwnedBookKeepsShelf(t *testing.T) {
	ctx := context.Background()
	owned, err := testApp.Services.Books.AddToLibrary(
		ctx, userID, learnProposal("IfAbsent Owned"), models.StatusReading, []string{},
	)
	require.NoError(t, err)

	ub, err := testApp.Services.Books.AddToLibraryIfAbsent(
		ctx, userID, learnProposal("IfAbsent Owned"), "To Learn",
	)
	require.NoError(t, err)
	assert.Equal(t, owned.BookID, ub.BookID)
	assert.Equal(t, models.StatusReading, ub.Status)
}

func TestEnsureLibraryBook_UnknownProviderNotFound(t *testing.T) {
	_, err := testApp.EnsureLibraryBook(
		context.Background(), userID, "no-such-provider", "123", "To Learn",
	)
	assert.ErrorIs(t, err, books.ErrExternalBookNotFound)
}

func TestGetLibraryBookByID_ReadBookReportsFullProgress(t *testing.T) {
	ub := addTestBookWithPages(t, "GetLibraryBookByID Read", 90)
	setStatus(t, ub, models.StatusRead)

	result, err := testApp.GetLibraryBookByID(context.Background(), userID, ub.BookID)
	require.NoError(t, err)
	assert.Equal(t, int32(100), result.ProgressPercent)
}
