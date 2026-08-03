package books_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindBookByTitleAndAuthor_NotFound exercises the repository method when no
// matching book exists — covers the 0-coverage path.
func TestFindBookByTitleAndAuthor_NotFound(t *testing.T) {
	book, err := testApp.Repositories.Books.FindBookByTitleAndAuthor(
		context.Background(),
		"nonexistent-title-xyz",
		"nonexistent-author-xyz",
	)
	require.Error(t, err)
	assert.Nil(t, book)
}

// TestFindBookByTitleAndAuthor_Found adds a book and then looks it up by title/author.
func TestFindBookByTitleAndAuthor_Found(t *testing.T) {
	ub := addTestBook(t, "FindByTitleBook")
	require.NotNil(t, ub)

	book, err := testApp.Repositories.Books.FindBookByTitleAndAuthor(
		context.Background(),
		"FindByTitleBook",
		"Test Author",
	)
	require.NoError(t, err)
	assert.NotNil(t, book)
	assert.Equal(t, "FindByTitleBook", book.Title)
}
