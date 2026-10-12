package repositories

import (
	"context"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

// ListEngaged returns the user's read, bookmarked or dismissed items with
// their feed's title, oldest first. Filtered items are the user's rules, not
// a reaction to the item, so they're left out.
func (repo *ItemsRepository) ListEngaged(
	ctx context.Context,
	userID string,
) ([]models.EngagedItem, error) {
	query := `
		SELECT i.id, i.title, f.title, i.categories, i.read_at,
		       i.dismissed, i.bookmarked, i.read_progress_pct
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.ingest_error IS NULL
		  AND i.filtered_at IS NULL
		  AND (i.read_at IS NOT NULL OR i.bookmarked OR i.dismissed)
		ORDER BY i.published_at
	`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.EngagedItem
	for rows.Next() {
		var item models.EngagedItem
		if err = rows.Scan(
			&item.ID, &item.Title, &item.FeedTitle, &item.Categories, &item.ReadAt,
			&item.Dismissed, &item.Bookmarked, &item.ReadProgressPct,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}
