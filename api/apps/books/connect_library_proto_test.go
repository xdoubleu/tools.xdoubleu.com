//nolint:testpackage // testing unexported package-level helpers
package books

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// TestCoverProxyURL_UsesBooksPrefix guards against the cover URL regressing
// back to the pre-rename "/backlog/" path: the cover route is mounted at
// "/books/api/cover/{bookId}" (see cover_routes.go), so the proxy URL built
// for clients must use that same prefix or covers 404.
func TestCoverProxyURL_UsesBooksPrefix(t *testing.T) {
	bookID := uuid.New()

	got := coverProxyURL(bookID, "http://api.test")

	assert.Equal(t, "http://api.test/books/api/cover/"+bookID.String(), got)
}

func TestProtoBook_CoverURLUsesBooksPrefix(t *testing.T) {
	cover := "https://openlibrary.org/cover.jpg"
	book := &models.Book{ //nolint:exhaustruct //optional nullable fields omitted
		ID:        uuid.New(),
		Title:     "Some Book",
		CoverURL:  &cover,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	pb := protoBook(book, "http://api.test")

	assert.Equal(t, "http://api.test/books/api/cover/"+book.ID.String(), pb.CoverUrl)
}

// TestReconcileOwnDigitalTag_StripsStaleTagWithNoFormats guards against a
// CSV-imported or otherwise stale own-digital tag surviving in the API
// response for a book with no attached PDF/EPUB.
func TestReconcileOwnDigitalTag_StripsStaleTagWithNoFormats(t *testing.T) {
	got := reconcileOwnDigitalTag([]string{"fantasy", models.TagOwnDigital}, nil)

	assert.Equal(t, []string{"fantasy"}, got)
}

// TestReconcileOwnDigitalTag_AddsMissingTagWithFormats guards against a
// book with an uploaded file failing to show as digitally owned because the
// tag write was skipped or lost.
func TestReconcileOwnDigitalTag_AddsMissingTagWithFormats(t *testing.T) {
	got := reconcileOwnDigitalTag([]string{"fantasy"}, []string{"pdf"})

	assert.ElementsMatch(t, []string{"fantasy", models.TagOwnDigital}, got)
}

func TestReconcileOwnDigitalTag_LeavesConsistentTagsUnchanged(t *testing.T) {
	withFormat := reconcileOwnDigitalTag(
		[]string{models.TagOwnDigital},
		[]string{"epub"},
	)
	assert.Equal(t, []string{models.TagOwnDigital}, withFormat)

	withoutFormat := reconcileOwnDigitalTag([]string{"fantasy"}, nil)
	assert.Equal(t, []string{"fantasy"}, withoutFormat)
}
