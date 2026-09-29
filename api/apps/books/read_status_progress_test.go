package books_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
)

func addTestBookWithPages(t *testing.T, title string, pages int) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //optional fields not needed
		Source:    "manual",
		Title:     title,
		Authors:   []string{"Test Author"},
		ISBN13:    testISBN(title),
		PageCount: pages,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	return ub
}

func setStatus(t *testing.T, ub *models.UserBook, status string) *models.UserBook {
	t.Helper()
	ctx := context.Background()
	ub.Status = status
	require.NoError(t, testApp.Services.Books.UpdateStatus(ctx, userID, *ub))
	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, ub.BookID)
	require.NoError(t, err)
	return saved
}

func TestReadStatus_PagesModeMovesToLastPage(t *testing.T) {
	ub := addTestBookWithPages(t, "ReadStatus Pages", 320)

	saved := setStatus(t, ub, models.StatusRead)

	assert.Equal(t, models.ProgressModePages, saved.ProgressMode)
	assert.Equal(t, 320, saved.CurrentPage)
	assert.Equal(t, 100, saved.ProgressPercent)
	assert.Equal(t, 100, saved.DisplayProgressPercent())
}

func TestReadStatus_UnknownPageCountSwitchesToPercent(t *testing.T) {
	ub := addTestBookNoISBN(t, "ReadStatus No Pages")

	saved := setStatus(t, ub, models.StatusRead)

	assert.Equal(t, models.ProgressModePercent, saved.ProgressMode)
	assert.Equal(t, 100, saved.DisplayProgressPercent())
}

func TestReadStatus_ProgressEditStaysFull(t *testing.T) {
	ctx := context.Background()
	ub := addTestBookWithPages(t, "ReadStatus Progress Edit", 200)
	setStatus(t, ub, models.StatusRead)

	require.NoError(t, testApp.Services.Books.UpdateProgress(
		ctx, userID, ub.BookID, models.ProgressModePercent, 0, 40,
	))

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, ub.BookID)
	require.NoError(t, err)
	assert.Equal(t, 100, saved.DisplayProgressPercent())
}

func TestReadStatus_LeavingReadStartsNewRead(t *testing.T) {
	ub := addTestBookWithPages(t, "ReadStatus Reread", 150)
	read := setStatus(t, ub, models.StatusRead)

	saved := setStatus(t, read, models.StatusReading)

	assert.Equal(t, 0, saved.CurrentPage)
	assert.Equal(t, 0, saved.ProgressPercent)
	assert.Equal(t, 0, saved.DisplayProgressPercent())
}

func TestReadStatus_AddedAsReadIsFull(t *testing.T) {
	ext := services.SourceProposal{ //nolint:exhaustruct //optional fields not needed
		Source:  "manual",
		Title:   "ReadStatus Added Read",
		Authors: []string{"Test Author"},
		ISBN13:  testISBN("ReadStatus Added Read"),
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusRead, []string{},
	)
	require.NoError(t, err)

	assert.Equal(t, 100, ub.DisplayProgressPercent())
}
