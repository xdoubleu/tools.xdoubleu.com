package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/kit/database/postgres"
)

// ListRankingCandidates returns the user's visible unread items plus those
// read since readSince (so a finished list stays finished), newest first,
// capped at limit.
func (repo *ItemsRepository) ListRankingCandidates(
	ctx context.Context,
	userID string,
	readSince time.Time,
	limit int,
) ([]models.Item, error) {
	items, _, err := repo.queryItemPage(ctx, limit, `
		SELECT `+itemListColumns+`
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.ingest_error IS NULL AND i.dismissed = false
		  AND i.filtered_at IS NULL AND i.expired_at IS NULL
		  AND (i.read_at IS NULL OR i.read_at >= $2)
		ORDER BY i.published_at DESC, i.created_at DESC
		LIMIT $3
	`, userID, readSince, limit+1)
	return items, err
}

// ReadStats returns the user's read-through counts since the given time, per
// feed and per lowercased category. Expired items are left out: unseen is
// not a negative signal.
func (repo *ItemsRepository) ReadStats(
	ctx context.Context,
	userID string,
	since time.Time,
) (map[uuid.UUID]models.ReadStat, map[string]models.ReadStat, error) {
	feedStats := map[uuid.UUID]models.ReadStat{}
	rows, err := repo.db.Query(ctx, `
		SELECT i.feed_id, count(*),
		    count(*) FILTER (WHERE i.read_at IS NOT NULL),
		    count(*) FILTER (WHERE i.bookmarked)
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.published_at >= $2
		  AND i.ingest_error IS NULL AND i.filtered_at IS NULL
		  AND i.expired_at IS NULL
		GROUP BY i.feed_id
	`, userID, since)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}
	for rows.Next() {
		var id uuid.UUID
		var st models.ReadStat
		if err = rows.Scan(&id, &st.Items, &st.Read, &st.Bookmarked); err != nil {
			rows.Close()
			return nil, nil, postgres.PgxErrorToHTTPError(err)
		}
		feedStats[id] = st
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}

	catStats := map[string]models.ReadStat{}
	crows, err := repo.db.Query(ctx, `
		SELECT lower(c.label), count(*),
		    count(*) FILTER (WHERE i.read_at IS NOT NULL),
		    count(*) FILTER (WHERE i.bookmarked)
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		CROSS JOIN LATERAL unnest(i.categories) AS c (label)
		WHERE f.user_id = $1 AND i.published_at >= $2
		  AND i.ingest_error IS NULL AND i.filtered_at IS NULL
		  AND i.expired_at IS NULL
		GROUP BY lower(c.label)
	`, userID, since)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}
	defer crows.Close()
	for crows.Next() {
		var key string
		var st models.ReadStat
		if err = crows.Scan(&key, &st.Items, &st.Read, &st.Bookmarked); err != nil {
			return nil, nil, postgres.PgxErrorToHTTPError(err)
		}
		catStats[key] = st
	}
	if err = crows.Err(); err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}
	return feedStats, catStats, nil
}
