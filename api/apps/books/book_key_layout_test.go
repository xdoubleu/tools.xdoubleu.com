package books_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUploadUsesPerBookKey verifies that a new upload is stored under the
// per-book folder (books/<bookID>/<checksum><ext>) rather than the old flat
// scheme (books/<checksum><ext>).
func TestUploadUsesPerBookKey(t *testing.T) {
	ub := seedBookInLibrary(
		t, userID, "PerBookKeyBook", "Homer", "9780140447934",
	)
	epubData := buildEPUBBytes("The Odyssey", "Homer", "9780140447934")

	result, err := uploadViaTestApp(
		t,
		userID,
		"odyssey.epub",
		"application/epub+zip",
		epubData,
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	// The storage key must be under the book's folder.
	expectedPrefix := "books/" + ub.BookID.String() + "/"
	assert.True(
		t,
		strings.HasPrefix(result.BookFile.StorageKey, expectedPrefix),
		"storage key %q should start with %q",
		result.BookFile.StorageKey,
		expectedPrefix,
	)
	// Exactly two slashes: "books/<bookID>/<checksum><ext>"
	slashes := strings.Count(result.BookFile.StorageKey, "/")
	assert.Equal(t, 2, slashes, "storage key should have exactly 2 '/' separators")
}
