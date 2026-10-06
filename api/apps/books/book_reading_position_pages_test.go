package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// pageTwoOffset is where page 2's anchor sits in PagedDocOne's KEPUB text.
const pageTwoOffset = len("Page one text.More of page one.")

func getPosition(t *testing.T, bookID string) *booksv1.ReadingPosition {
	t.Helper()
	req := connect.NewRequest(&booksv1.GetReadingStateRequest{BookId: bookID})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetReadingState(context.Background(), req)
	require.NoError(t, err)
	return resp.Msg.State.Position
}

func translate(
	t *testing.T, bookID string, pos *booksv1.ReadingPosition,
) (*booksv1.ReadingPosition, error) {
	t.Helper()
	req := connect.NewRequest(&booksv1.TranslateReadingPositionRequest{
		BookId: bookID, Position: pos,
	})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).TranslateReadingPosition(
		context.Background(), req,
	)
	if err != nil {
		return nil, err
	}
	return resp.Msg.Position, nil
}

func TestConnectGetReadingState_PDFSourced_BothForms(t *testing.T) {
	bookID := setupPagedBook(t, userID)
	webPage(t, userID, bookID, 2)

	pos := getPosition(t, bookID.String())
	require.NotNil(t, pos)
	assert.Equal(t, int32(2), pos.Page)
	assert.Equal(t, mocks.PagedDocOne, pos.Href)
	assert.Equal(t, int32(pageTwoOffset), pos.Offset)

	regeneratePaged(t, userID, bookID)
	pos = getPosition(t, bookID.String())
	assert.Equal(t, int32(2), pos.Page, "the stored page survives regeneration")
	assert.Equal(t, int32(pageTwoOffset+len("Intro paragraph.")), pos.Offset,
		"and points at page 2 in the new KEPUB")
}

func TestConnectTranslateReadingPosition(t *testing.T) {
	bookID := setupPagedBook(t, userID).String()

	got, err := translate(t, bookID, &booksv1.ReadingPosition{Page: 3})
	require.NoError(t, err)
	assert.Equal(t, int32(3), got.Page)
	assert.Equal(t, mocks.PagedDocTwo, got.Href)
	assert.Equal(t, int32(0), got.Offset)

	got, err = translate(t, bookID,
		&booksv1.ReadingPosition{Href: mocks.PagedDocOne, Offset: int32(pageTwoOffset + 3)})
	require.NoError(t, err)
	assert.Equal(t, int32(2), got.Page, "the page whose anchor precedes it")
	assert.Equal(t, mocks.PagedDocOne, got.Href)

	epubOnly := addUniqueBook(t).ID.String()
	got, err = translate(t, epubOnly, &booksv1.ReadingPosition{Page: 4})
	require.NoError(t, err)
	assert.Equal(t, &booksv1.ReadingPosition{Page: 4}, got, "nothing to translate")

	for _, bad := range []struct {
		id  string
		pos *booksv1.ReadingPosition
	}{
		{"not-a-uuid", &booksv1.ReadingPosition{Page: 1}},
		{bookID, nil},
		{bookID, &booksv1.ReadingPosition{Href: "a.xhtml", Page: 2}},
	} {
		_, err = translate(t, bad.id, bad.pos)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), bad.id)
	}
}
