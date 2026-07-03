package books_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// addUniqueBook inserts a book with no ISBN so each call gets a distinct UUID
// across test runs (no upsert collision).
func addUniqueBook(t *testing.T) *models.Book {
	t.Helper()
	book, err := testApp.Repositories.Books.UpsertBook(
		context.Background(),
		models.Book{ //nolint:exhaustruct //only required fields; no ISBN to avoid sharing
			Title:   fmt.Sprintf("unique-book-%s", uuid.NewString()),
			Authors: []string{"Test Author"},
		},
	)
	require.NoError(t, err)
	return book
}

func TestBookFilesRepo_Insert(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatEPUB,
		StorageKey: "users/test/books/abc/file.epub",
		SizeBytes:  1024,
		Status:     models.FileStatusReady,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)
	require.NotNil(t, inserted)
	assert.Equal(t, f.BookID, inserted.BookID)
	assert.Equal(t, f.UserID, inserted.UserID)
	assert.Equal(t, models.FileFormatEPUB, inserted.Format)
	assert.Equal(t, models.FileStatusReady, inserted.Status)
	assert.NotEqual(t, uuid.Nil, inserted.ID)
}

func TestBookFilesRepo_GetByID_Found(t *testing.T) {
	book := addUniqueBook(t)

	chk := "sha256-abc"
	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatPDF,
		StorageKey: "users/test/books/abc/file.pdf",
		SizeBytes:  2048,
		Checksum:   &chk,
		Status:     models.FileStatusReady,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		inserted.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
	assert.Equal(t, &chk, got.Checksum)
}

func TestBookFilesRepo_GetByID_NotFound(t *testing.T) {
	got, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		uuid.New(),
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_ListByBook(t *testing.T) {
	book := addUniqueBook(t)

	for _, format := range []string{models.FileFormatPDF, models.FileFormatEPUB} {
		f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     book.ID,
			UserID:     userID,
			Format:     format,
			StorageKey: "users/test/books/" + format + "/file." + format,
			SizeBytes:  512,
			Status:     models.FileStatusReady,
		}
		_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
		require.NoError(t, err)
	}

	files, err := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	for _, f := range files {
		assert.Equal(t, book.ID, f.BookID)
		assert.Equal(t, userID, f.UserID)
	}
}

func TestBookFilesRepo_ListByBook_Empty(t *testing.T) {
	book := addUniqueBook(t)

	files, err := testApp.Repositories.BookFiles.ListByBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestBookFilesRepo_GetByBookAndFormat_Found(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatKEPUB,
		StorageKey: "users/test/books/kepub/file.kepub",
		SizeBytes:  4096,
		Status:     models.FileStatusConverting,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.GetByBookAndFormat(
		context.Background(), userID, book.ID, models.FileFormatKEPUB,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
	assert.Equal(t, models.FileStatusConverting, got.Status)
}

func TestBookFilesRepo_GetByBookAndFormat_NotFound(t *testing.T) {
	got, err := testApp.Repositories.BookFiles.GetByBookAndFormat(
		context.Background(), userID, uuid.New(), models.FileFormatEPUB,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_UpdateStatus(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatKEPUB,
		StorageKey: "users/test/books/kepub/update.kepub",
		SizeBytes:  1024,
		Status:     models.FileStatusConverting,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	err = testApp.Repositories.BookFiles.UpdateStatus(
		context.Background(), inserted.ID, models.FileStatusReady,
	)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		inserted.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, got.Status)
}

func TestBookFilesRepo_Delete(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatPDF,
		StorageKey: "users/test/books/pdf/delete.pdf",
		SizeBytes:  256,
		Status:     models.FileStatusReady,
	}

	inserted, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	err = testApp.Repositories.BookFiles.Delete(context.Background(), inserted.ID)
	require.NoError(t, err)

	got, err := testApp.Repositories.BookFiles.GetByID(
		context.Background(),
		inserted.ID,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Nil(t, got)
}

func TestBookFilesRepo_FormatsByUser_PDFOnly(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatPDF,
		StorageKey: "users/test/books/pdf/formats.pdf",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	result, err := testApp.Repositories.BookFiles.FormatsByUser(
		context.Background(),
		userID,
	)
	require.NoError(t, err)
	assert.Contains(t, result[book.ID], models.FileFormatPDF)
	assert.NotContains(t, result[book.ID], models.FileFormatEPUB)
}

func TestBookFilesRepo_FormatsByUser_EPUBOnly(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatEPUB,
		StorageKey: "users/test/books/epub/formats.epub",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	result, err := testApp.Repositories.BookFiles.FormatsByUser(
		context.Background(),
		userID,
	)
	require.NoError(t, err)
	assert.Contains(t, result[book.ID], models.FileFormatEPUB)
	assert.NotContains(t, result[book.ID], models.FileFormatPDF)
}

func TestBookFilesRepo_FormatsByUser_BothFormats(t *testing.T) {
	book := addUniqueBook(t)

	for _, format := range []string{models.FileFormatPDF, models.FileFormatEPUB} {
		f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
			BookID:     book.ID,
			UserID:     userID,
			Format:     format,
			StorageKey: "users/test/books/" + format + "/both." + format,
			SizeBytes:  512,
			Status:     models.FileStatusReady,
		}
		_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
		require.NoError(t, err)
	}

	result, err := testApp.Repositories.BookFiles.FormatsByUser(
		context.Background(),
		userID,
	)
	require.NoError(t, err)
	assert.ElementsMatch(
		t,
		[]string{models.FileFormatEPUB, models.FileFormatPDF},
		result[book.ID],
	)
}

func TestBookFilesRepo_FormatsByUser_KEPUBExcluded(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatKEPUB,
		StorageKey: "users/test/books/kepub/excluded.kepub",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	result, err := testApp.Repositories.BookFiles.FormatsByUser(
		context.Background(),
		userID,
	)
	require.NoError(t, err)
	assert.NotContains(t, result, book.ID)
}

func TestBookFilesRepo_FormatsByUser_NotReadyExcluded(t *testing.T) {
	book := addUniqueBook(t)

	f := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.ID,
		UserID:     userID,
		Format:     models.FileFormatEPUB,
		StorageKey: "users/test/books/epub/converting.epub",
		SizeBytes:  512,
		Status:     models.FileStatusConverting,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), f)
	require.NoError(t, err)

	result, err := testApp.Repositories.BookFiles.FormatsByUser(
		context.Background(),
		userID,
	)
	require.NoError(t, err)
	assert.NotContains(t, result, book.ID)
}
