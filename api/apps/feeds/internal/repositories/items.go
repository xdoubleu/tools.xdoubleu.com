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

// ItemsRepository stores ingested feed entries and each feed's seen set.
type ItemsRepository struct {
	db postgres.DB
}

// itemColumns includes the article body; only GetByIDForUser may use it.
const itemColumns = `i.id, i.feed_id, i.guid, i.title, i.source_url,
	i.content_html, i.published_at, i.read_at, i.dismissed, i.bookmarked,
	i.read_progress_pct, i.ingest_error, i.created_at`

// itemListColumns replaces the body with a has-content boolean. Every
// multi-row read and RETURNING clause must use it: bodies are tens of MB per
// page and exhaust database egress.
const itemListColumns = `i.id, i.feed_id, i.guid, i.title, i.source_url,
	i.content_html <> '', i.published_at, i.read_at, i.dismissed, i.bookmarked,
	i.read_progress_pct, i.ingest_error, i.created_at`

// scanItem scans an itemColumns row, deriving HasContent.
func scanItem(row pgx.Row) (*models.Item, error) {
	item, err := scanItemInto(row, func(i *models.Item) any { return &i.ContentHTML })
	if err != nil {
		return nil, err
	}
	item.HasContent = item.ContentHTML != ""
	return item, nil
}

// scanListItem scans an itemListColumns row, leaving ContentHTML empty.
func scanListItem(row pgx.Row) (*models.Item, error) {
	return scanItemInto(row, func(i *models.Item) any { return &i.HasContent })
}

// scanItemInto holds the column order shared by both column lists.
func scanItemInto(
	row pgx.Row,
	contentTarget func(*models.Item) any,
) (*models.Item, error) {
	var item models.Item
	err := row.Scan(
		&item.ID,
		&item.FeedID,
		&item.GUID,
		&item.Title,
		&item.SourceURL,
		contentTarget(&item),
		&item.PublishedAt,
		&item.ReadAt,
		&item.Dismissed,
		&item.Bookmarked,
		&item.ReadProgressPct,
		&item.IngestError,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FilterNewGUIDs returns the subset of guids with no feeds.items row yet,
// preserving input order.
func (repo *ItemsRepository) FilterNewGUIDs(
	ctx context.Context,
	feedID uuid.UUID,
	guids []string,
) ([]string, error) {
	if len(guids) == 0 {
		return nil, nil
	}
	query := `
		SELECT g.guid
		FROM unnest($2::text[]) WITH ORDINALITY AS g (guid, ord)
		WHERE NOT EXISTS (
		    SELECT 1 FROM feeds.items i
		    WHERE i.feed_id = $1 AND i.guid = g.guid
		)
		ORDER BY g.ord
	`
	rows, err := repo.db.Query(ctx, query, feedID, guids)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var g string
		if scanErr := rows.Scan(&g); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, g)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// Insert stores an item, or an error row with ingest_error set. A duplicate
// (feed_id, guid) re-opens the existing item instead: polling pre-filters
// seen guids, so this only happens when an email is resent on purpose.
func (repo *ItemsRepository) Insert(
	ctx context.Context,
	item models.Item,
) error {
	var publishedAt *time.Time
	if !item.PublishedAt.IsZero() {
		publishedAt = &item.PublishedAt
	}

	query := `
		INSERT INTO feeds.items
			(feed_id, guid, title, source_url, content_html, published_at,
			 ingest_error)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()), $7)
		ON CONFLICT (feed_id, guid) DO UPDATE SET
			read_at = NULL,
			dismissed = false
	`
	_, err := repo.db.Exec(
		ctx, query,
		item.FeedID, item.GUID, item.Title, item.SourceURL, item.ContentHTML,
		publishedAt, item.IngestError,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// Update partially updates an item, scoped to its owner; nil leaves a column
// unchanged. read sets or clears read_at; readProgressPct only increases.
// Returns database.ErrResourceNotFound when no owned item matches.
func (repo *ItemsRepository) Update(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	read, dismissed, bookmarked *bool,
	readProgressPct *int32,
) (*models.Item, error) {
	query := `
		UPDATE feeds.items i
		SET read_at = CASE
		        WHEN $3::bool IS NULL THEN i.read_at
		        WHEN $3::bool THEN now()
		        ELSE NULL
		      END,
		    dismissed = COALESCE($4, i.dismissed),
		    bookmarked = COALESCE($5, i.bookmarked),
		    read_progress_pct = GREATEST(
		        i.read_progress_pct, COALESCE($6, i.read_progress_pct)
		    )
		FROM feeds.feeds f
		WHERE i.feed_id = f.id AND f.user_id = $1 AND i.id = $2
		RETURNING ` + itemListColumns
	item, err := scanListItem(repo.db.QueryRow(
		ctx, query, userID, itemID, read, dismissed, bookmarked, readProgressPct,
	))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return item, nil
}

// GetByIDForUser returns one owned item including its body — the only read
// of content_html. Returns database.ErrResourceNotFound when none matches.
func (repo *ItemsRepository) GetByIDForUser(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) (*models.Item, error) {
	query := `
		SELECT ` + itemColumns + `
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.id = $2
	`
	item, err := scanItem(repo.db.QueryRow(ctx, query, userID, itemID))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return item, nil
}

// ListByUser returns a page of the user's non-dismissed, successfully
// ingested items, newest first, excluding error/skip dedup markers.
// unreadOnly, feedID and bookmarkedOnly narrow the results.
func (repo *ItemsRepository) ListByUser(
	ctx context.Context,
	userID string,
	limit, offset int32,
	unreadOnly bool,
	feedID *uuid.UUID,
	bookmarkedOnly bool,
) ([]models.Item, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	query := `
		SELECT ` + itemListColumns + `
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.ingest_error IS NULL AND i.dismissed = false
		  AND ($4::bool = false OR i.read_at IS NULL)
		  AND ($5::uuid IS NULL OR i.feed_id = $5)
		  AND ($6::bool = false OR i.bookmarked = true)
		ORDER BY i.published_at DESC, i.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := repo.db.Query(
		ctx, query, userID, sqlLimit, offset, unreadOnly, feedID, bookmarkedOnly,
	)
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

// CountUnread counts the user's non-dismissed, ingested, unread items.
func (repo *ItemsRepository) CountUnread(
	ctx context.Context,
	userID string,
) (int, error) {
	var count int
	err := repo.db.QueryRow(ctx, `
		SELECT count(*)
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.ingest_error IS NULL AND i.dismissed = false
		  AND i.read_at IS NULL
	`, userID).Scan(&count)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	return count, nil
}

// CountUnreadByFeed counts non-dismissed, ingested, unread items per feed
// across all users, omitting feeds with none.
func (repo *ItemsRepository) CountUnreadByFeed(
	ctx context.Context,
) ([]models.FeedUnreadCount, error) {
	query := `
		SELECT f.id, f.title, f.url, count(i.id)
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE i.ingest_error IS NULL AND i.dismissed = false
		  AND i.read_at IS NULL
		GROUP BY f.id, f.user_id, f.title, f.url
		ORDER BY f.user_id, f.title, f.url
	`
	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FeedUnreadCount
	for rows.Next() {
		var c models.FeedUnreadCount
		// url is nullable: email feeds have none.
		var url *string
		if scanErr := rows.Scan(
			&c.FeedID, &c.FeedTitle, &url, &c.UnreadCount,
		); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		if url != nil {
			c.FeedURL = *url
		}
		out = append(out, c)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// RecentPublishedAt returns the publish times of the feed's most recent
// ingested items, newest first.
func (repo *ItemsRepository) RecentPublishedAt(
	ctx context.Context,
	feedID uuid.UUID,
	limit int,
) ([]time.Time, error) {
	query := `
		SELECT published_at
		FROM feeds.items
		WHERE feed_id = $1 AND ingest_error IS NULL
		ORDER BY published_at DESC
		LIMIT $2
	`
	rows, err := repo.db.Query(ctx, query, feedID, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var t time.Time
		if scanErr := rows.Scan(&t); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, t)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// Stats aggregates per feed: item count, average interval (0 below 2 items),
// read rate, and average read completion.
func (repo *ItemsRepository) Stats(
	ctx context.Context,
	userID string,
) ([]models.FeedStats, error) {
	query := `
		WITH gaps AS (
			SELECT
				feed_id,
				EXTRACT(EPOCH FROM (
					published_at - LAG(published_at) OVER (
						PARTITION BY feed_id ORDER BY published_at
					)
				)) / 3600.0 AS gap_hours
			FROM feeds.items
			WHERE ingest_error IS NULL
		),
		avg_gaps AS (
			SELECT feed_id, AVG(gap_hours) AS avg_interval_hours
			FROM gaps
			WHERE gap_hours IS NOT NULL
			GROUP BY feed_id
		)
		SELECT
			f.id,
			f.title,
			COUNT(i.id),
			COALESCE(ag.avg_interval_hours, 0),
			COALESCE(AVG(CASE WHEN i.read_at IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(i.read_progress_pct), 0)
		FROM feeds.feeds f
		LEFT JOIN feeds.items i ON i.feed_id = f.id AND i.ingest_error IS NULL
		LEFT JOIN avg_gaps ag ON ag.feed_id = f.id
		WHERE f.user_id = $1
		GROUP BY f.id, f.title, ag.avg_interval_hours
		ORDER BY f.title, f.url
	`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FeedStats
	for rows.Next() {
		var s models.FeedStats
		if scanErr := rows.Scan(
			&s.FeedID, &s.FeedTitle, &s.ItemCount,
			&s.AvgIntervalHours, &s.ReadRate, &s.AvgReadProgressPct,
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

// ItemsPerDay buckets the user's items by ingest day since the given time;
// created_at is used since published_at can be backdated or missing.
func (repo *ItemsRepository) ItemsPerDay(
	ctx context.Context,
	userID string,
	since time.Time,
) ([]models.DayCount, error) {
	query := `
		SELECT date_trunc('day', i.created_at) AS day, COUNT(*)
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1 AND i.ingest_error IS NULL AND i.created_at >= $2
		GROUP BY day
		ORDER BY day
	`
	rows, err := repo.db.Query(ctx, query, userID, since)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.DayCount
	for rows.Next() {
		var d models.DayCount
		if scanErr := rows.Scan(&d.Day, &d.Count); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, d)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}
