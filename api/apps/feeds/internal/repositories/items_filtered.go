package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

// scanFilteredItem scans an itemListColumns row followed by the LEFT JOINed
// rule's columns, which are NULL once the rule is deleted.
func scanFilteredItem(row pgx.Row) (*models.Item, error) {
	var (
		ruleID        *uuid.UUID
		ruleFeedID    *uuid.UUID
		kind, value   *string
		ruleCreatedAt *time.Time
	)
	item, err := scanItemInto(
		row,
		func(i *models.Item) any { return &i.HasContent },
		&ruleID, &ruleFeedID, &kind, &value, &ruleCreatedAt,
	)
	if err != nil || ruleID == nil {
		return item, err
	}
	//nolint:exhaustruct // UserID/FilteredCount aren't needed here
	item.FilterRule = &models.FilterRule{
		ID:        *ruleID,
		FeedID:    ruleFeedID,
		Kind:      *kind,
		Value:     *value,
		CreatedAt: *ruleCreatedAt,
	}
	return item, nil
}

// ListFilteredByUser returns a page of the user's filtered items with their
// rule, most recently filtered first, optionally for one feed.
func (repo *ItemsRepository) ListFilteredByUser(
	ctx context.Context,
	userID string,
	limit, offset int32,
	feedID *uuid.UUID,
) ([]models.Item, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	query := `
		SELECT ` + itemListColumns + `,
		    r.id, r.feed_id, r.kind, r.value, r.created_at
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		LEFT JOIN feeds.filter_rules r ON r.id = i.filtered_rule_id
		WHERE f.user_id = $1 AND i.filtered_at IS NOT NULL
		  AND ($4::uuid IS NULL OR i.feed_id = $4)
		ORDER BY i.filtered_at DESC, i.id
		LIMIT $2 OFFSET $3
	`
	rows, err := repo.db.Query(ctx, query, userID, sqlLimit, offset, feedID)
	if err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.Item
	for rows.Next() {
		item, scanErr := scanFilteredItem(rows)
		if scanErr != nil {
			return nil, false, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *item)
	}
	if err = rows.Err(); err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}

	page, hasMore := pagination.Split(out, safeLimit)
	return page, hasMore, nil
}

// Restore returns one of the user's filtered items to the inbox as unread,
// marking it restored; a non-empty contentHTML replaces its body. Returns
// database.ErrResourceNotFound when no owned filtered item matches.
func (repo *ItemsRepository) Restore(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	contentHTML string,
) (*models.Item, error) {
	query := `
		UPDATE feeds.items i
		SET filtered_at = NULL, filtered_rule_id = NULL, restored_at = now(),
		    read_at = NULL,
		    content_html = CASE WHEN $3 = '' THEN i.content_html ELSE $3 END
		FROM feeds.feeds f
		WHERE i.feed_id = f.id AND f.user_id = $1 AND i.id = $2
		  AND i.filtered_at IS NOT NULL
		RETURNING ` + itemListColumns
	item, err := scanListItem(repo.db.QueryRow(ctx, query, userID, itemID, contentHTML))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return item, nil
}
