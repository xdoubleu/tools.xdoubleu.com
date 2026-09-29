package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// ListSuggestions returns, per feed and category (grouped ignoring case),
// the user's items published since `since` that are ingested, unfiltered
// and never restored, where there are at least minItems and at most
// maxReadPct percent are read. Categories a category rule covers (the
// feed's or a global one) or the user dismissed are left out. Least read
// first.
func (repo *FilterRulesRepository) ListSuggestions(
	ctx context.Context,
	userID string,
	since time.Time,
	minItems, maxReadPct int,
) ([]models.FilterRuleSuggestion, error) {
	query := `
		WITH counts AS (
		    SELECT i.feed_id, lower(c.label) AS key, min(c.label) AS category,
		        count(*) AS item_count,
		        count(*) FILTER (WHERE i.read_at IS NOT NULL) AS read_count
		    FROM feeds.items i
		    JOIN feeds.feeds f ON f.id = i.feed_id
		    CROSS JOIN LATERAL unnest(i.categories) AS c (label)
		    WHERE f.user_id = $1 AND i.published_at >= $2
		      AND i.ingest_error IS NULL AND i.filtered_at IS NULL
		      AND i.restored_at IS NULL
		    GROUP BY i.feed_id, lower(c.label)
		)
		SELECT s.feed_id, f.title, COALESCE(f.url, ''), s.category,
		    s.item_count, s.read_count
		FROM counts s
		JOIN feeds.feeds f ON f.id = s.feed_id
		WHERE s.item_count >= $3 AND s.read_count * 100 <= s.item_count * $4
		  AND NOT EXISTS (
		      SELECT 1 FROM feeds.filter_rules r
		      WHERE r.user_id = $1 AND r.kind = 'category'
		        AND (r.feed_id IS NULL OR r.feed_id = s.feed_id)
		        AND lower(r.value) = s.key
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM feeds.dismissed_rule_suggestions d
		      WHERE d.user_id = $1 AND d.feed_id = s.feed_id
		        AND lower(d.category) = s.key
		  )
		ORDER BY s.read_count::float8 / s.item_count, s.item_count DESC,
		    f.title, s.key
	`
	rows, err := repo.db.Query(ctx, query, userID, since, minItems, maxReadPct)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FilterRuleSuggestion
	for rows.Next() {
		var s models.FilterRuleSuggestion
		if scanErr := rows.Scan(
			&s.FeedID, &s.FeedTitle, &s.FeedURL, &s.Category,
			&s.ItemCount, &s.ReadCount,
		); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, s)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// DismissSuggestion records that the user doesn't want feedID's category
// suggested again; dismissing twice is a no-op. Returns
// database.ErrResourceNotFound when feedID isn't the user's.
func (repo *FilterRulesRepository) DismissSuggestion(
	ctx context.Context,
	userID string,
	feedID uuid.UUID,
	category string,
) error {
	query := `
		WITH f AS (
		    SELECT id FROM feeds.feeds WHERE id = $2 AND user_id = $1
		),
		ins AS (
		    INSERT INTO feeds.dismissed_rule_suggestions
		        (user_id, feed_id, category)
		    SELECT $1, f.id, $3 FROM f
		    ON CONFLICT (user_id, feed_id, lower(category)) DO NOTHING
		)
		SELECT EXISTS (SELECT 1 FROM f)
	`
	var owned bool
	if err := repo.db.QueryRow(ctx, query, userID, feedID, category).
		Scan(&owned); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if !owned {
		return database.ErrResourceNotFound
	}
	return nil
}
