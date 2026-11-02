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

// ExpireDue hides every expirable item that arrived more than its feed's
// lifetime ago and returns how many it hid.
func (repo *ItemsRepository) ExpireDue(ctx context.Context) (int64, error) {
	tag, err := repo.db.Exec(ctx, `
		UPDATE feeds.items i
		SET expired_at = now()
		FROM feeds.feeds f
		WHERE i.feed_id = f.id AND `+expirableCond+`
		  AND i.created_at + make_interval(days => f.expiry_days) <= now()
	`)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected(), nil
}

// queryItemPage runs a list query whose rows are itemListColumns and
// splits off the has-more probe row.
func (repo *ItemsRepository) queryItemPage(
	ctx context.Context,
	safeLimit int,
	query string,
	args ...any,
) ([]models.Item, bool, error) {
	rows, err := repo.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.Item
	for rows.Next() {
		item, scanErr := scanListItem(rows)
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

// ListExpiredByUser returns a page of the user's expired items, most
// recently expired first, optionally for one feed.
func (repo *ItemsRepository) ListExpiredByUser(
	ctx context.Context,
	userID string,
	limit, offset int32,
	feedID *uuid.UUID,
) ([]models.Item, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)
	query := `
		SELECT ` + itemListColumns + `
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.expired_at IS NOT NULL
		  AND ($4::uuid IS NULL OR i.feed_id = $4)
		ORDER BY i.expired_at DESC, i.id
		LIMIT $2 OFFSET $3
	`
	return repo.queryItemPage(ctx, safeLimit, query, userID, sqlLimit, offset, feedID)
}

// RestoreExpired returns one of the user's expired items to the inbox as
// unread; it never expires again. Returns database.ErrResourceNotFound when
// no owned expired item matches.
func (repo *ItemsRepository) RestoreExpired(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) (*models.Item, error) {
	query := `
		UPDATE feeds.items i
		SET expired_at = NULL, restored_at = now()
		FROM feeds.feeds f
		WHERE i.feed_id = f.id AND f.user_id = $1 AND i.id = $2
		  AND i.expired_at IS NOT NULL
		RETURNING ` + itemListColumns
	item, err := scanListItem(repo.db.QueryRow(ctx, query, userID, itemID))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return item, nil
}

// ListLastChance returns the user's items expiring within window, soonest
// first; feedID narrows it to one feed.
func (repo *ItemsRepository) ListLastChance(
	ctx context.Context,
	userID string,
	limit, offset int32,
	window time.Duration,
	feedID *uuid.UUID,
) ([]models.Item, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)
	query := `
		SELECT ` + itemListColumns + `
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND ` + expirableCond + `
		  AND i.created_at + make_interval(days => f.expiry_days)
		      <= now() + make_interval(secs => $5)
		  AND ($4::uuid IS NULL OR i.feed_id = $4)
		ORDER BY i.created_at + make_interval(days => f.expiry_days), i.id
		LIMIT $2 OFFSET $3
	`
	return repo.queryItemPage(
		ctx, safeLimit, query,
		userID, sqlLimit, offset, feedID, window.Seconds(),
	)
}

func scanDigestItems(rows pgx.Rows) ([]models.DigestItem, error) {
	defer rows.Close()
	var out []models.DigestItem
	for rows.Next() {
		var d models.DigestItem
		if err := rows.Scan(&d.Title, &d.FeedTitle, &d.URL, &d.ExpiresAt); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// DigestNewest returns the newest unread items across all users.
func (repo *ItemsRepository) DigestNewest(
	ctx context.Context,
	limit int,
) ([]models.DigestItem, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT i.title, f.title, i.source_url, `+expiresAtExpr+`
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE i.ingest_error IS NULL AND i.dismissed = false
		  AND i.read_at IS NULL AND i.filtered_at IS NULL
		  AND i.expired_at IS NULL
		ORDER BY i.published_at DESC, i.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return scanDigestItems(rows)
}

// DigestLastChance returns the unread items across all users that expire
// within window, soonest first.
func (repo *ItemsRepository) DigestLastChance(
	ctx context.Context,
	limit int,
	window time.Duration,
) ([]models.DigestItem, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT i.title, f.title, i.source_url, `+expiresAtExpr+`
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE `+expirableCond+`
		  AND i.created_at + make_interval(days => f.expiry_days)
		      <= now() + make_interval(secs => $2)
		ORDER BY i.created_at + make_interval(days => f.expiry_days), i.id
		LIMIT $1
	`, limit, window.Seconds())
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return scanDigestItems(rows)
}
