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

// ItemsRepository stores ingested feed entries (feeds.items) — a feed's own
// seen-item set and its content in one place, since items no longer link out
// to a separate library.
type ItemsRepository struct {
	db postgres.DB
}

// itemColumns reads the full row including the article body. Only
// GetByIDForUser uses it — the body is the single widest column in the
// schema, so anything returning more than one row must use itemListColumns
// instead (issue #1027).
const itemColumns = `i.id, i.feed_id, i.guid, i.title, i.source_url,
	i.content_html, i.published_at, i.read_at, i.dismissed, i.bookmarked,
	i.read_progress_pct, i.ingest_error, i.created_at`

// itemListColumns is itemColumns with the article body replaced by a
// boolean saying whether there is one, mirroring books' bookColumns
// (apps/books/internal/repositories/books_scan.go). Every multi-row read and
// every RETURNING clause uses this: a 50-item page carried tens of MB of
// extracted HTML out of Postgres on each request otherwise, which is what
// exhausted the egress quota in issue #1027.
const itemListColumns = `i.id, i.feed_id, i.guid, i.title, i.source_url,
	i.content_html <> '', i.published_at, i.read_at, i.dismissed, i.bookmarked,
	i.read_progress_pct, i.ingest_error, i.created_at`

// scanItem scans a row selected with itemColumns (body included), deriving
// HasContent so callers see the same field set either way.
func scanItem(row pgx.Row) (*models.Item, error) {
	item, err := scanItemInto(row, func(i *models.Item) any { return &i.ContentHTML })
	if err != nil {
		return nil, err
	}
	item.HasContent = item.ContentHTML != ""
	return item, nil
}

// scanListItem scans a row selected with itemListColumns (body replaced by
// the has-content boolean), leaving ContentHTML empty.
func scanListItem(row pgx.Row) (*models.Item, error) {
	return scanItemInto(row, func(i *models.Item) any { return &i.HasContent })
}

// scanItemInto holds the column order shared by both column lists, which
// differ only in what the sixth column is.
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

// Insert stores one ingested item (or a metadata-only/error row when ingest
// failed — content_html/source_url/title may be empty and ingest_error set).
// A duplicate (feed_id, guid) re-opens the existing item (clears read/
// dismissed) instead of inserting — polling never retries a seen guid (it
// pre-filters via FilterNewGUIDs), but resending the same email reuses the
// same guid and is the intended way to bring a dismissed/read item back.
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

// Update partially updates an item's read/dismissed/bookmarked/read-progress
// state, scoped to the owning user via a join on feeds.feeds — nil pointers
// leave the corresponding column unchanged. read, when non-nil, sets read_at
// to now() (true) or clears it (false). readProgressPct only ever increases
// (GREATEST) — re-opening and scrolling less never lowers the recorded
// completion (issue #798). Returns database.ErrResourceNotFound when no item
// matches (unknown id or owned by another user).
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

// GetByIDForUser returns one item including its article body, scoped to the
// owning user via the same join on feeds.feeds the other queries use. This
// is the only read that touches content_html, so the reader pays for one
// body when it opens an article instead of every list read paying for fifty
// (issue #1027). Returns database.ErrResourceNotFound when no item matches
// (unknown id or owned by another user).
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

// ListByUser returns non-dismissed, successfully ingested items from any of
// userID's feeds, newest first, paginated by limit/offset (see
// pagination.Clamp). Error/skip dedup markers (ingest_error set, no title or
// content) are excluded — they exist only so polling doesn't retry a guid,
// not for display. unreadOnly, when true, excludes items with a set read_at.
// feedID, when non-nil, restricts results to that one feed. bookmarkedOnly,
// when true, excludes items without bookmarked set.
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

// CountUnread returns the number of non-dismissed, successfully ingested,
// unread items across any of userID's feeds — the reading dashboard's feeds
// widget shows this alongside a few recent items from ListByUser.
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

// CountUnreadByFeed returns the number of non-dismissed, successfully
// ingested, unread items per feed, across all users, restricted to feeds
// with at least one such item — for the weekly digest job's open-feed-items
// reminder (issue #1355). Only ids/counts are selected, never content_html
// (see the "Never put a wide TEXT column in a list query" convention).
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
		// url is nullable (email feeds have none, see feedColumns' own
		// handling in feeds.go's scanFeed).
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

// RecentPublishedAt returns the publish timestamps of the feed's most recent
// successfully-ingested items, newest first, for the quiet-feed cadence
// check (issue #799).
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

// Stats aggregates posting cadence and read/completion metrics per feed
// (issue #798): item count, average interval between items (0 when fewer
// than 2), read rate (fraction with read_at set), and average read
// completion percentage.
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

// ItemsPerDay buckets item ingest counts by day, across all of the user's
// feeds, since the given time — the "when do new items appear" histogram
// (issue #798). created_at (ingest time) is used rather than published_at,
// which can be backdated or missing on some feeds.
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
