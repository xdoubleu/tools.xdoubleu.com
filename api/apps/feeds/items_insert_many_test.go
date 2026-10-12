package feeds_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/repositories"
)

func TestInsertManyStoresRowsAndReopensDuplicates(t *testing.T) {
	ctx := context.Background()
	repo := repositories.New(testDB).Items

	seeded := seedFeedItem(t, userID, "InsertMany Seed Post")
	var feedID uuid.UUID
	var guid string
	require.NoError(t, testDB.QueryRow(
		ctx, "SELECT feed_id, guid FROM feeds.items WHERE id = $1", seeded,
	).Scan(&feedID, &guid))
	_, err := testDB.Exec(ctx,
		"UPDATE feeds.items SET read_at = now(), dismissed = true WHERE id = $1",
		seeded)
	require.NoError(t, err)

	require.NoError(t, repo.InsertMany(ctx, nil))

	ingestErr := "skipped: over per-poll cap"
	prefix := uuid.NewString()
	published := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	//nolint:exhaustruct // only the stored columns matter
	items := []models.Item{
		{
			FeedID: feedID, GUID: prefix + "-a", Title: "A",
			SourceURL: "https://example.com/a", ContentHTML: "<p>a</p>",
			PublishedAt: published, Categories: []string{"x", "y"},
		},
		{FeedID: feedID, GUID: prefix + "-b", IngestError: &ingestErr},
		{FeedID: feedID, GUID: guid},
	}
	require.NoError(t, repo.InsertMany(ctx, items))

	var title, sourceURL, html string
	var pub time.Time
	var cats []string
	require.NoError(t, testDB.QueryRow(ctx, `
		SELECT title, source_url, content_html, published_at, categories
		FROM feeds.items WHERE feed_id = $1 AND guid = $2`,
		feedID, prefix+"-a",
	).Scan(&title, &sourceURL, &html, &pub, &cats))
	assert.Equal(t, "A", title)
	assert.Equal(t, "https://example.com/a", sourceURL)
	assert.Equal(t, "<p>a</p>", html)
	assert.True(t, published.Equal(pub))
	assert.Equal(t, []string{"x", "y"}, cats)

	var gotErr *string
	var bCats []string
	require.NoError(t, testDB.QueryRow(ctx,
		"SELECT ingest_error, categories FROM feeds.items WHERE feed_id=$1 AND guid=$2",
		feedID, prefix+"-b").Scan(&gotErr, &bCats))
	require.NotNil(t, gotErr)
	assert.Equal(t, ingestErr, *gotErr)
	assert.Empty(t, bCats)

	var readAt *time.Time
	var dismissed bool
	require.NoError(t, testDB.QueryRow(ctx,
		"SELECT read_at, dismissed FROM feeds.items WHERE id = $1", seeded,
	).Scan(&readAt, &dismissed))
	assert.Nil(t, readAt, "duplicate re-opens the item")
	assert.False(t, dismissed)
}

func TestInsertManyChunksLargeBatches(t *testing.T) {
	ctx := context.Background()
	repo := repositories.New(testDB).Items

	seeded := seedFeedItem(t, userID, "InsertMany Chunk Post")
	var feedID uuid.UUID
	require.NoError(t, testDB.QueryRow(
		ctx, "SELECT feed_id FROM feeds.items WHERE id = $1", seeded,
	).Scan(&feedID))

	prefix := uuid.NewString()
	items := make([]models.Item, 0, 7000)
	for i := range 7000 {
		//nolint:exhaustruct // only the stored columns matter
		items = append(items, models.Item{
			FeedID: feedID, GUID: prefix + strconv.Itoa(i), Title: "t",
		})
	}
	require.NoError(t, repo.InsertMany(ctx, items))

	var n int
	require.NoError(t, testDB.QueryRow(ctx,
		"SELECT count(*) FROM feeds.items WHERE feed_id=$1 AND guid LIKE $2",
		feedID, prefix+"%").Scan(&n))
	assert.Equal(t, 7000, n)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.Error(t, repo.InsertMany(cancelled, items[:1]))
}
