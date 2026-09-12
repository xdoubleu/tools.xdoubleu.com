package books_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
)

// TestGetLibraryBookByID_Owner confirms GetLibraryBookByID — the exported
// method the learningpaths app (#1474) uses to resolve a resource linked to
// a books entry — returns the caller's own library entry.
func TestGetLibraryBookByID_Owner(t *testing.T) {
	ub := addTestBookNoISBN(t, "GetLibraryBookByID Owner")

	result, err := testApp.GetLibraryBookByID(context.Background(), userID, ub.BookID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ub.BookID.String(), result.BookId)
	assert.Equal(t, "GetLibraryBookByID Owner", result.Book.GetTitle())
}

// TestGetLibraryBookByID_OtherUserNotFound confirms a book that exists but
// belongs to a different user 404s rather than leaking its existence or
// data — the same "404 on foreign ownership" rule used everywhere else in
// this repo for user-scoped lookups.
func TestGetLibraryBookByID_OtherUserNotFound(t *testing.T) {
	ub := addTestBookNoISBN(t, "GetLibraryBookByID Foreign")

	_, err := testApp.GetLibraryBookByID(
		context.Background(), "a-different-user", ub.BookID,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// TestGetLibraryBookByID_MissingNotFound confirms a book ID that doesn't
// exist at all also 404s.
func TestGetLibraryBookByID_MissingNotFound(t *testing.T) {
	_, err := testApp.GetLibraryBookByID(context.Background(), userID, uuid.New())
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}
