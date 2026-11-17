package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/repositories"
)

func TestNotificationOutboxLifecycle(t *testing.T) {
	repo := repositories.NewNotificationOutboxRepository(testDB)
	ctx := t.Context()
	_, err := testDB.Exec(ctx, `DELETE FROM global.notification_outbox`)
	require.NoError(t, err)

	first, err := repo.Enqueue(ctx, "a@b.c", "s1", "b")
	require.NoError(t, err)
	second, err := repo.Enqueue(ctx, "", "s2", "b")
	require.NoError(t, err)

	now := time.Now()
	claimed, err := repo.ClaimDue(ctx, now, time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 2)
	assert.Equal(t, first, claimed[0].ID)
	assert.Equal(t, 1, claimed[0].Attempts)
	assert.Equal(t, "a@b.c", claimed[0].To)

	again, err := repo.ClaimDue(ctx, now, time.Minute, 10)
	require.NoError(t, err)
	assert.Empty(t, again, "a leased row is hidden from other claimers")

	require.NoError(t, repo.MarkSent(ctx, first))
	require.NoError(t, repo.MarkRetry(ctx, second, now.Add(-time.Second), "boom"))

	retried, err := repo.ClaimDue(ctx, now, time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	assert.Equal(t, 2, retried[0].Attempts)

	require.NoError(t, repo.MarkFailed(ctx, second, "gave up"))
	depth, err := repo.Depth(ctx)
	require.NoError(t, err)
	assert.Equal(t, models.OutboxDepth{Pending: 0, Failed: 1}, depth)

	var lastErr string
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT last_error FROM global.notification_outbox WHERE id = $1`, second,
	).Scan(&lastErr))
	assert.Equal(t, "gave up", lastErr)

	require.NoError(t, repo.PruneFinished(ctx, time.Now().Add(time.Minute)))
	depth, err = repo.Depth(ctx)
	require.NoError(t, err)
	assert.Equal(t, models.OutboxDepth{Pending: 0, Failed: 0}, depth)
}
