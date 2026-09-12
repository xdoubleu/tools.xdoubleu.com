package learningpaths_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

// seedLibraryBook stages a books library entry owned by owner directly via
// SQL, mirroring how other apps' own tests seed cross-schema fixtures
// without importing an internal package they can't reach.
func seedLibraryBook(t *testing.T, owner, title string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var bookID uuid.UUID
	err := testDB.QueryRow(ctx, `
		INSERT INTO books.books (title) VALUES ($1) RETURNING id
	`, title).Scan(&bookID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO books.user_books (user_id, book_id, status)
		VALUES ($1, $2, 'to_read')
	`, owner, bookID)
	require.NoError(t, err)

	return bookID
}

// seedFeedItem stages a feeds item owned by owner directly via SQL.
func seedFeedItem(t *testing.T, owner, title string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var feedID uuid.UUID
	err := testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES (gen_random_uuid(), $1, $2, $3, 'rss')
		RETURNING id
	`, owner, "https://example.com/"+title, title+" Feed").Scan(&feedID)
	require.NoError(t, err)

	var itemID uuid.UUID
	err = testDB.QueryRow(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, published_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id
	`, feedID, title+"-guid", title, "https://example.com/"+title+"/article").
		Scan(&itemID)
	require.NoError(t, err)

	return itemID
}

func TestCreateLearningPath_LinkedBookResolved(t *testing.T) {
	client := setupClient(getRoutes())
	bookID := seedLibraryBook(t, userID, "Linked Book")

	resp, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Linked Book",
			Resources: []*learningpathsv1.Resource{
				{LinkedBookId: ptrTo(bookID.String())},
			},
		}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.LearningPath.Resources, 1)
	res := resp.Msg.LearningPath.Resources[0]
	assert.Equal(t, bookID.String(), res.GetLinkedBookId())
	require.NotNil(t, res.LinkedBook)
	assert.Equal(t, "Linked Book", res.LinkedBook.Title)
	assert.Equal(t, "to_read", res.LinkedBook.Status)
}

func TestCreateLearningPath_LinkedFeedItemResolved(t *testing.T) {
	client := setupClient(getRoutes())
	itemID := seedFeedItem(t, userID, "Linked Item")

	resp, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Linked Feed Item",
			Resources: []*learningpathsv1.Resource{
				{LinkedFeedItemId: ptrTo(itemID.String())},
			},
		}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.LearningPath.Resources, 1)
	res := resp.Msg.LearningPath.Resources[0]
	assert.Equal(t, itemID.String(), res.GetLinkedFeedItemId())
	require.NotNil(t, res.LinkedFeedItem)
	assert.Equal(t, "Linked Item", res.LinkedFeedItem.Title)
}

// TestCreateLearningPath_LinkedBookForeignOwnerRejected confirms a
// linked_book_id belonging to a different user is rejected up front
// (InvalidArgument) rather than silently persisted — the cross-user-data
// leak this app must never allow (#1474 "Not this: no write access to
// books/feeds data ... read-only linking" plus the general 404-on-foreign-
// ownership rule extended to write-time validation).
func TestCreateLearningPath_LinkedBookForeignOwnerRejected(t *testing.T) {
	client := setupClient(getRoutes())
	bookID := seedLibraryBook(t, "a-different-user", "Foreign Book")

	_, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Foreign Book",
			Resources: []*learningpathsv1.Resource{
				{LinkedBookId: ptrTo(bookID.String())},
			},
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

// TestCreateLearningPath_LinkedFeedItemForeignOwnerRejected is the feeds
// counterpart to TestCreateLearningPath_LinkedBookForeignOwnerRejected.
func TestCreateLearningPath_LinkedFeedItemForeignOwnerRejected(t *testing.T) {
	client := setupClient(getRoutes())
	itemID := seedFeedItem(t, "a-different-user", "Foreign Item")

	_, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Foreign Item",
			Resources: []*learningpathsv1.Resource{
				{LinkedFeedItemId: ptrTo(itemID.String())},
			},
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

// TestCreateLearningPath_LinkedBookUnknownIDRejected confirms a well-formed
// but nonexistent linked_book_id is rejected the same way a foreign-owned
// one is.
func TestCreateLearningPath_LinkedBookUnknownIDRejected(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Unknown Book",
			Resources: []*learningpathsv1.Resource{
				{LinkedBookId: ptrTo(uuid.New().String())},
			},
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUpdateLearningPath_LinkedBookForeignOwnerRejected(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path To Update With Foreign Book",
		}),
	)
	require.NoError(t, err)

	bookID := seedLibraryBook(t, "a-different-user", "Foreign Update Book")

	_, err = client.UpdateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.UpdateLearningPathRequest{
			Id:    createResp.Msg.LearningPath.Id,
			Title: "Updated",
			Resources: []*learningpathsv1.Resource{
				{LinkedBookId: ptrTo(bookID.String())},
			},
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

// TestCreateLearningPath_MalformedLinkedIDsTreatedAsAbsent confirms a
// linked_book_id/linked_feed_item_id that isn't a parseable UUID is silently
// dropped by dtoToResources' parseOptionalUUID rather than rejected —
// per its doc comment, a well-formed but nonexistent/foreign-owned ID is
// still caught downstream by validateResourceLinks, but a malformed string
// never reaches that check at all.
func TestCreateLearningPath_MalformedLinkedIDsTreatedAsAbsent(t *testing.T) {
	client := setupClient(getRoutes())

	resp, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Path With Malformed Link IDs",
			Resources: []*learningpathsv1.Resource{
				{
					Text:             "still has text",
					LinkedBookId:     ptrTo("not-a-valid-uuid"),
					LinkedFeedItemId: ptrTo("also-not-a-valid-uuid"),
				},
			},
		}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.LearningPath.Resources, 1)
	res := resp.Msg.LearningPath.Resources[0]
	assert.Equal(t, "still has text", res.Text)
	assert.Nil(t, res.LinkedBookId)
	assert.Nil(t, res.LinkedFeedItemId)
	assert.Nil(t, res.LinkedBook)
	assert.Nil(t, res.LinkedFeedItem)
}

func ptrTo[T any](v T) *T {
	return &v
}
