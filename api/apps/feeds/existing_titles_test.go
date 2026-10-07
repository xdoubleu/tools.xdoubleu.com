package feeds_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/repositories"
)

func TestExistingTitles(t *testing.T) {
	ctx := context.Background()
	repo := repositories.New(testDB).Items

	itemID := seedFeedItem(t, userID, "ExistingTitles Known Post")
	var feedID uuid.UUID
	require.NoError(t, testDB.QueryRow(
		ctx, "SELECT feed_id FROM feeds.items WHERE id = $1", itemID,
	).Scan(&feedID))

	empty, err := repo.ExistingTitles(ctx, feedID, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	found, err := repo.ExistingTitles(
		ctx, feedID, []string{"existingtitles known post", "unseen post"},
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"existingtitles known post": true}, found)

	other, err := repo.ExistingTitles(
		ctx, uuid.New(), []string{"existingtitles known post"},
	)
	require.NoError(t, err)
	assert.Empty(t, other, "titles are scoped to the feed")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = repo.ExistingTitles(cancelled, feedID, []string{"x"})
	require.Error(t, err)
}
