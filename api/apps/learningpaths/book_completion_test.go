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

// seedLibraryBookWithProgress stages a books library entry owned by owner via
// SQL with the given progress_percent.
func seedLibraryBookWithProgress(
	t *testing.T, owner string, title string, progressPercent int32,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var bookID uuid.UUID
	err := testDB.QueryRow(ctx, `
		INSERT INTO books.books (title) VALUES ($1) RETURNING id
	`, title).Scan(&bookID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO books.user_books
		(user_id, book_id, status, progress_mode, progress_percent)
		VALUES ($1, $2, 'reading', 'percent', $3)
	`, owner, bookID, progressPercent)
	require.NoError(t, err)

	return bookID
}

// updateBookProgress rewrites a book's progress_percent, simulating the
// reading app advancing it between reads.
func updateBookProgress(t *testing.T, bookID uuid.UUID, progressPercent int32) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		UPDATE books.user_books SET progress_percent = $1 WHERE book_id = $2
	`, progressPercent, bookID)
	require.NoError(t, err)
}

// deletePath removes a path, keeping the shared test user's DB clean so
// connect_test's TestListLearningPaths_Empty stays green regardless of run
// order.
func deletePath(t *testing.T, pathID string) {
	_, err := testDB.Exec(context.Background(), `
		DELETE FROM learningpaths.learning_paths WHERE id = $1
	`, pathID)
	require.NoError(t, err)
}

// TestBookLinkedItem_PartialThenCompleteRoundsTrip: a book-linked item reads
// complete only when the linked book is at 100%; below that it's incomplete,
// ignoring any stored manual value. Seeds the book below, confirms, advances
// to 100%, confirms it flips. This is backfill-aware: the item is created
// (as a fresh create would) and then the book's progress is updated, the same
// shape existing rows predating a migration would take.
func TestBookLinkedItem_PartialThenCompleteRoundsTrip(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	bookID := seedLibraryBookWithProgress(t, userID, "Book Completion", 40)

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Reading Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{
						{
							Description:  "Read the book",
							LinkedBookId: ptrTo(bookID.String()),
						},
					},
				},
			},
		}),
	)
	require.NoError(t, err)
	item := createResp.Msg.LearningPath.Modules[0].Items[0]
	defer deletePath(t, createResp.Msg.LearningPath.Id)
	assert.False(t, item.Completed)
	assert.Equal(t, bookID.String(), item.GetLinkedBookId())
	require.NotNil(t, item.LinkedBook)
	assert.Equal(t, "Book Completion", item.LinkedBook.Title)
	assert.Equal(t, int32(40), item.LinkedBook.ProgressPercent)

	// A manual toggle on a book-linked item is rejected; completion is derived.
	_, err = client.RecordItemProgress(
		ctx, connect.NewRequest(&learningpathsv1.RecordItemProgressRequest{
			ItemId: item.Id, Completed: true,
		}),
	)
	require.Error(t, err)

	// Advance the book to 100% — the item flips complete with no manual toggle.
	updateBookProgress(t, bookID, 100)
	getResp, err := client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.True(t, getResp.Msg.LearningPath.Modules[0].Items[0].Completed)
}

// TestProgressCountsReflectBookLinkedItems: GetLearningPathProgress derives
// completion from the linked book, so a partial book doesn't count toward
// completed items but a finished one does.
func TestProgressCountsReflectBookLinkedItems(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	bookID := seedLibraryBookWithProgress(t, userID, "Progress Book", 100)
	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Counting Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{
						{
							Description:  "automatic",
							LinkedBookId: ptrTo(bookID.String()),
						},
						{Description: "manual, uncompleted"},
					},
				},
			},
		}),
	)
	require.NoError(t, err)
	defer deletePath(t, createResp.Msg.LearningPath.Id)

	resp, err := client.GetLearningPathProgress(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathProgressRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Msg.TotalItems)
	assert.Equal(t, int32(1), resp.Msg.CompletedItems)
}

// TestBookLinkedItem_NonBookItemsKeepManualToggle: items without a linked
// book keep their stored completed flag and support RecordItemProgress.
func TestBookLinkedItem_NonBookItemsKeepManualToggle(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Manual Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{{Description: "manual item"}},
				},
			},
		}),
	)
	require.NoError(t, err)
	itemID := createResp.Msg.LearningPath.Modules[0].Items[0].Id
	defer deletePath(t, createResp.Msg.LearningPath.Id)

	_, err = client.RecordItemProgress(
		ctx, connect.NewRequest(&learningpathsv1.RecordItemProgressRequest{
			ItemId: itemID, Completed: true,
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.True(t, getResp.Msg.LearningPath.Modules[0].Items[0].Completed)
}

// TestBookLinkedItem_UnknownBookRejected: a linked_book_id that doesn't
// resolve is rejected at authoring, never persisted.
func TestBookLinkedItem_UnknownBookRejected(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Bad Book Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{
						{Description: "bad", LinkedBookId: ptrTo(uuid.New().String())},
					},
				},
			},
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
