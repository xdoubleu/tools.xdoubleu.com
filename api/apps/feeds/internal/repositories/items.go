package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// ItemsRepository stores ingested feed entries (feeds.items) — a feed's own
// seen-item set and its content in one place, since items no longer link out
// to a separate library.
type ItemsRepository struct {
	db postgres.DB
}

const itemColumns = `i.id, i.feed_id, i.guid, i.title, i.source_url,
	i.content_html, i.published_at, i.read_at, i.dismissed, i.favourite,
	i.ingest_error, i.created_at`

func scanItem(row pgx.Row) (*models.Item, error) {
	var item models.Item
	err := row.Scan(
		&item.ID,
		&item.FeedID,
		&item.GUID,
		&item.Title,
		&item.SourceURL,
		&item.ContentHTML,
		&item.PublishedAt,
		&item.ReadAt,
		&item.Dismissed,
		&item.Favourite,
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
// A duplicate (feed_id, guid) is a no-op: seen items are never retried by
// polling.
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
		ON CONFLICT (feed_id, guid) DO NOTHING
	`
	_, err := repo.db.Exec(
		ctx, query,
		item.FeedID, item.GUID, item.Title, item.SourceURL, item.ContentHTML,
		publishedAt, item.IngestError,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// Update partially updates an item's read/dismissed/favourite state, scoped
// to the owning user via a join on feeds.feeds — nil pointers leave the
// corresponding column unchanged. read, when non-nil, sets read_at to now()
// (true) or clears it (false). Returns database.ErrResourceNotFound when no
// item matches (unknown id or owned by another user).
func (repo *ItemsRepository) Update(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	read, dismissed, favourite *bool,
) (*models.Item, error) {
	query := `
		UPDATE feeds.items i
		SET read_at = CASE
		        WHEN $3::bool IS NULL THEN i.read_at
		        WHEN $3::bool THEN now()
		        ELSE NULL
		      END,
		    dismissed = COALESCE($4, i.dismissed),
		    favourite = COALESCE($5, i.favourite)
		FROM feeds.feeds f
		WHERE i.feed_id = f.id AND f.user_id = $1 AND i.id = $2
		RETURNING ` + itemColumns
	item, err := scanItem(repo.db.QueryRow(
		ctx, query, userID, itemID, read, dismissed, favourite,
	))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return item, nil
}

// ListByUser returns every item ingested by any of userID's feeds, newest
// first.
func (repo *ItemsRepository) ListByUser(
	ctx context.Context,
	userID string,
) ([]models.Item, error) {
	query := `
		SELECT ` + itemColumns + `
		FROM feeds.items i
		JOIN feeds.feeds f ON f.id = i.feed_id
		WHERE f.user_id = $1
		ORDER BY i.published_at DESC, i.created_at DESC
	`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.Item
	for rows.Next() {
		item, scanErr := scanItem(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *item)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}
