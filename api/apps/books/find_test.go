package books_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBook(t *testing.T) {
	ctx := context.Background()
	const uid = "books-find-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM books.user_books WHERE user_id = $1", uid)
	require.NoError(t, err)

	m, err := testApp.FindBook(ctx, uid, "Odyssey", "Homer")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "The Odyssey", m.Title)
	assert.Equal(t, "Homer", m.Creator)
	assert.Equal(t, "hardcover:9780140447934", m.Ref)
	assert.Equal(t, "https://hardcover.app/cover.jpg", m.ImageURL)
	assert.False(t, m.Known)

	_, err = testApp.EnsureLibraryBook(ctx, uid, "hardcover", "9780140447934", "to-read")
	require.NoError(t, err)
	m, err = testApp.FindBook(ctx, uid, "The Odyssey", "")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.True(t, m.Known)

	m, err = testApp.FindBook(ctx, uid, "Some Other Book", "Homer")
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestFindBook_LibraryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testApp.FindBook(ctx, "books-find-test-user", "Odyssey", "Homer")
	assert.Error(t, err)
}
