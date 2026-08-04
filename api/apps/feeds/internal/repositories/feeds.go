package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// FeedsRepository stores RSS/Atom subscriptions and email-relay newsletter
// subscriptions (feeds.feeds).
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

// List returns the user's feeds ordered by title, then URL (untitled feeds
// sort together instead of first).
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

// ListAll returns every pollable (rss/scrape) feed across all users, for the
// background poll job — email feeds are push-only and never polled.
func (repo *FeedsRepository) ListAll(ctx context.Context) ([]models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM feeds.feeds
		WHERE source_type IN ('rss', 'scrape')
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

// Insert creates a feed. A duplicate (user_id, url) maps to
// database.ErrResourceConflict via the unique constraint; for email feeds
// (url is empty) a duplicate inbound_token hash maps to the same error via
// the feeds_inbound_token_idx unique index.
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

// GetByInboundTokenHash resolves an email feed by its inbound alias token's
// SHA-256 hash, for the unauthenticated Resend webhook handler — there is no
// user_id to scope by at that point.
// Returns database.ErrResourceNotFound when no feed matches.
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

// Update changes the user-editable fields (title).
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

// Delete removes the feed; its items cascade (feeds.items.feed_id ON DELETE
// CASCADE) — unlike reading's former FeedsRepository, there is no library
// linkage to preserve, so deleting a feed always deletes every item it
// ingested, read/favourited or not.
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

// SetFetchResult records the outcome of a poll: the conditional-GET
// validators on success (fetchErr nil), or the error message on failure
// (validators kept so an intermittently failing feed still short-circuits
// once it recovers unchanged). last_fetched_at is always bumped.
// consecutive_failures increments on failure and resets to 0 on success —
// the returned row lets the caller act on the post-update streak (issue
// #799) without a second round trip.
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

// MarkNotified records that a problem email has been sent for this feed, so
// SetFetchResult's caller doesn't re-send while the problem persists (issue
// #799). Only called after mailer.Client.SendTo succeeds — a failed send is
// retried on the next poll.
func (repo *FeedsRepository) MarkNotified(ctx context.Context, id uuid.UUID) error {
	_, err := repo.db.Exec(
		ctx, `UPDATE feeds.feeds SET notified_at = now() WHERE id = $1`, id,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// ClearNotified clears a feed's outstanding-problem marker once it recovers
// (issue #799).
func (repo *FeedsRepository) ClearNotified(ctx context.Context, id uuid.UUID) error {
	_, err := repo.db.Exec(
		ctx, `UPDATE feeds.feeds SET notified_at = NULL WHERE id = $1`, id,
	)
	return postgres.PgxErrorToHTTPError(err)
}
