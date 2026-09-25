package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// FeedsRepository stores feed subscriptions (feeds.feeds).
type FeedsRepository struct {
	db postgres.DB
}

const feedColumns = `id, user_id, url, title, source_type,
	inbound_token, etag, last_modified, last_fetched_at, last_error,
	consecutive_failures, notified_at, created_at, updated_at`

func scanFeed(row pgx.Row) (*models.Feed, error) {
	var f models.Feed
	var url *string
	err := row.Scan(
		&f.ID,
		&f.UserID,
		&url,
		&f.Title,
		&f.SourceType,
		&f.InboundToken,
		&f.ETag,
		&f.LastModified,
		&f.LastFetchedAt,
		&f.LastError,
		&f.ConsecutiveFailures,
		&f.NotifiedAt,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if url != nil {
		f.URL = *url
	}
	return &f, nil
}

// List returns the user's feeds ordered by title, then URL.
func (repo *FeedsRepository) List(
	ctx context.Context,
	userID string,
) ([]models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE user_id = $1
		ORDER BY title, url
	`
	return repo.queryFeeds(ctx, query, userID)
}

// ListAll returns every pollable (rss/scrape) feed across all users.
func (repo *FeedsRepository) ListAll(ctx context.Context) ([]models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE source_type IN ('rss', 'scrape')
		ORDER BY user_id, title, url
	`
	return repo.queryFeeds(ctx, query)
}

// ListUnhealthy returns every failing feed across all users.
func (repo *FeedsRepository) ListUnhealthy(ctx context.Context) ([]models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE consecutive_failures > 0
		ORDER BY user_id, title, url
	`
	return repo.queryFeeds(ctx, query)
}

func (repo *FeedsRepository) queryFeeds(
	ctx context.Context,
	query string,
	args ...any,
) ([]models.Feed, error) {
	rows, err := repo.db.Query(ctx, query, args...)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.Feed
	for rows.Next() {
		f, scanErr := scanFeed(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *f)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// GetByID returns a single feed scoped to its owner.
// Returns database.ErrResourceNotFound when no feed matches.
func (repo *FeedsRepository) GetByID(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (*models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE user_id = $1 AND id = $2
	`
	f, err := scanFeed(repo.db.QueryRow(ctx, query, userID, id))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// Insert creates a feed. A duplicate (user_id, url), or inbound_token for
// email feeds, maps to database.ErrResourceConflict.
func (repo *FeedsRepository) Insert(
	ctx context.Context,
	feed models.Feed,
) (*models.Feed, error) {
	var url *string
	if feed.URL != "" {
		url = &feed.URL
	}
	sourceType := feed.SourceType
	if sourceType == "" {
		sourceType = models.FeedSourceRSS
	}

	query := `
		INSERT INTO feeds.feeds
			(user_id, url, title, source_type, inbound_token)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + feedColumns

	f, err := scanFeed(repo.db.QueryRow(
		ctx, query,
		feed.UserID, url, feed.Title, sourceType, feed.InboundToken,
	))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// GetByInboundTokenHash resolves an email feed by its token hash, unscoped
// (the webhook has no user). Returns database.ErrResourceNotFound if none.
func (repo *FeedsRepository) GetByInboundTokenHash(
	ctx context.Context,
	hash string,
) (*models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE inbound_token = $1
	`
	f, err := scanFeed(repo.db.QueryRow(ctx, query, hash))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// Update changes the feed's title.
func (repo *FeedsRepository) Update(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	title string,
) error {
	query := `
		UPDATE feeds.feeds
		SET title = $3
		WHERE user_id = $1 AND id = $2
	`
	tag, err := repo.db.Exec(ctx, query, userID, id, title)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// Delete removes the feed; all its items cascade.
func (repo *FeedsRepository) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	query := `DELETE FROM feeds.feeds WHERE user_id = $1 AND id = $2`
	tag, err := repo.db.Exec(ctx, query, userID, id)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// SetFetchResult records a poll outcome: validators on success, the error on
// failure (validators kept). It bumps last_fetched_at and updates
// consecutive_failures, returning the row so the caller sees the streak.
func (repo *FeedsRepository) SetFetchResult(
	ctx context.Context,
	id uuid.UUID,
	etag, lastModified, fetchErr *string,
) (*models.Feed, error) {
	query := `
		UPDATE feeds.feeds
		SET etag          = COALESCE($2, etag),
		    last_modified = COALESCE($3, last_modified),
		    last_error    = $4::text,
		    consecutive_failures = CASE
		        WHEN $4::text IS NULL THEN 0
		        ELSE consecutive_failures + 1
		    END,
		    last_fetched_at = now()
		WHERE id = $1
		RETURNING ` + feedColumns
	f, err := scanFeed(repo.db.QueryRow(ctx, query, id, etag, lastModified, fetchErr))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// MarkNotified records a sent problem email; called only after a successful
// send, so a failed one retries next poll.
func (repo *FeedsRepository) MarkNotified(ctx context.Context, id uuid.UUID) error {
	_, err := repo.db.Exec(
		ctx, `UPDATE feeds.feeds SET notified_at = now() WHERE id = $1`, id,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// ClearNotified clears a feed's outstanding-problem marker.
func (repo *FeedsRepository) ClearNotified(ctx context.Context, id uuid.UUID) error {
	_, err := repo.db.Exec(
		ctx, `UPDATE feeds.feeds SET notified_at = NULL WHERE id = $1`, id,
	)
	return postgres.PgxErrorToHTTPError(err)
}
