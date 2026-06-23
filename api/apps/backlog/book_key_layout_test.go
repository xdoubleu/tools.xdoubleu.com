package backlog_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/backlog/internal/jobs"
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

// TestRelocateFilesJob migrates a synthetic flat-key row to the per-book scheme.
func TestRelocateFilesJob(t *testing.T) {
	// Upload a file via the current path — this creates a per-book row.
	seedBookInLibrary(t, userID, "RelocationBook", "Homer", "9780140229126")
	epubData := buildEPUBBytes("The Iliad", "Homer", "9780140229126")

	result, err := uploadViaTestApp(
		t, userID, "iliad.epub", "application/epub+zip", epubData,
	)
	require.NoError(t, err)

	// Manually rewrite the storage_key to simulate a legacy flat-key row.
	checksum := ""
	if result.BookFile.Checksum != nil {
		checksum = *result.BookFile.Checksum
	}
	flatKey := "books/" + checksum + ".epub"
	require.NoError(t, fakeStore.Put(
		context.Background(),
		flatKey,
		bytes.NewReader(epubData),
		int64(len(epubData)),
		"application/epub+zip",
	))
	require.NoError(t, testApp.Repositories.BookFiles.UpdateStorageKey(
		context.Background(),
		result.BookFile.ID,
		flatKey,
	))

	// Run the relocate job.
	job := jobs.NewRelocateFilesJob(testApp.Services.Books)
	err = job.Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)

	// The row's storage_key should now be under the per-book folder.
	updated, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		result.BookFile.ID,
	)
	require.NoError(t, err)
	expectedPrefix := "books/" + result.UserBook.BookID.String() + "/"
	assert.True(
		t,
		strings.HasPrefix(updated.StorageKey, expectedPrefix),
		"after relocation, storage key %q should start with %q",
		updated.StorageKey,
		expectedPrefix,
	)
}
