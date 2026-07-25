package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// FeedsRepository stores RSS/Atom subscriptions (reading.feeds) and their
// per-feed seen-item set (reading.feed_items).
type FeedsRepository struct {
	db postgres.DB
}

const feedColumns = `id, user_id, url, title, kobo_sync, etag, last_modified,
	last_fetched_at, last_error, created_at, updated_at`

func scanFeed(row pgx.Row) (*models.Feed, error) {
	var f models.Feed
	err := row.Scan(
		&f.ID,
		&f.UserID,
		&f.URL,
		&f.Title,
		&f.KoboSync,
		&f.ETag,
		&f.LastModified,
		&f.LastFetchedAt,
		&f.LastError,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, err
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
		FROM reading.feeds
		WHERE user_id = $1
		ORDER BY title, url
	`
	return repo.queryFeeds(ctx, query, userID)
}

// ListAll returns every feed across all users, for the background poll job.
func (repo *FeedsRepository) ListAll(ctx context.Context) ([]models.Feed, error) {
	query := `
		SELECT ` + feedColumns + `
		FROM reading.feeds
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
		FROM reading.feeds
		WHERE user_id = $1 AND id = $2
	`
	f, err := scanFeed(repo.db.QueryRow(ctx, query, userID, id))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// Insert creates a feed. A duplicate (user_id, url) maps to
// database.ErrResourceConflict via the unique constraint.
func (repo *FeedsRepository) Insert(
	ctx context.Context,
	feed models.Feed,
) (*models.Feed, error) {
	query := `
		INSERT INTO reading.feeds (user_id, url, title, kobo_sync)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + feedColumns

	f, err := scanFeed(repo.db.QueryRow(
		ctx, query, feed.UserID, feed.URL, feed.Title, feed.KoboSync,
	))
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return f, nil
}

// Update changes the user-editable fields (title, kobo_sync).
func (repo *FeedsRepository) Update(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	title string,
	koboSync bool,
) error {
	query := `
		UPDATE reading.feeds
		SET title = $3, kobo_sync = $4
		WHERE user_id = $1 AND id = $2
	`
	tag, err := repo.db.Exec(ctx, query, userID, id, title, koboSync)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// ListRemovableBookIDs returns the book IDs this feed ingested that the user
// has NOT engaged with — items still unread and not favourited. Read or
// favourited items are excluded so they survive the feed's deletion. Scoped to
// a feed the user owns.
func (repo *FeedsRepository) ListRemovableBookIDs(
	ctx context.Context,
	userID string,
	feedID uuid.UUID,
) ([]uuid.UUID, error) {
	query := `
		SELECT fi.book_id
		FROM reading.feed_items fi
		JOIN reading.feeds f ON f.id = fi.feed_id AND f.user_id = $1
		JOIN reading.user_books ub
		    ON ub.book_id = fi.book_id AND ub.user_id = $1
		WHERE fi.feed_id = $2
		    AND fi.book_id IS NOT NULL
		    AND ub.status <> 'read'
		    AND NOT ('favourite' = ANY(ub.tags))
	`
	rows, err := repo.db.Query(ctx, query, userID, feedID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, id)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// ListItemBooks returns, for every book this user's feeds have ingested into
// the library, which feed it came from — used to label the ad hoc
// feed-reader view. Books whose feed has since been deleted (but were kept
// because the user engaged with them) are omitted, since feed_items cascades
// away with the feed.
func (repo *FeedsRepository) ListItemBooks(
	ctx context.Context,
	userID string,
) ([]models.FeedItemBook, error) {
	query := `
		SELECT fi.book_id, f.id, f.title
		FROM reading.feed_items fi
		JOIN reading.feeds f ON f.id = fi.feed_id
		WHERE f.user_id = $1 AND fi.book_id IS NOT NULL
	`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FeedItemBook
	for rows.Next() {
		var item models.FeedItemBook
		if scanErr := rows.Scan(&item.BookID, &item.FeedID, &item.FeedTitle); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// Delete removes the feed; its feed_items rows cascade. Library items already
// ingested from the feed are not touched (the service layer removes the
// unengaged ones first — see FeedService.Delete).
func (repo *FeedsRepository) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	query := `DELETE FROM reading.feeds WHERE user_id = $1 AND id = $2`
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
func (repo *FeedsRepository) SetFetchResult(
	ctx context.Context,
	id uuid.UUID,
	etag, lastModified, fetchErr *string,
) error {
	query := `
		UPDATE reading.feeds
		SET etag          = COALESCE($2, etag),
		    last_modified = COALESCE($3, last_modified),
		    last_error    = $4,
		    last_fetched_at = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(ctx, query, id, etag, lastModified, fetchErr)
	return postgres.PgxErrorToHTTPError(err)
}

// FilterNewGUIDs returns the subset of guids with no feed_items row yet,
// preserving input order.
func (repo *FeedsRepository) FilterNewGUIDs(
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
		    SELECT 1 FROM reading.feed_items fi
		    WHERE fi.feed_id = $1 AND fi.guid = g.guid
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

// MarkItemSeen records a guid as processed for the feed, with either the
// ingested book or the per-item ingest error. Seen items are never retried
// by polling (Add-by-URL is the manual retry path), so this is a no-op on
// conflict.
func (repo *FeedsRepository) MarkItemSeen(
	ctx context.Context,
	feedID uuid.UUID,
	guid string,
	bookID *uuid.UUID,
	ingestErr *string,
) error {
	query := `
		INSERT INTO reading.feed_items (feed_id, guid, book_id, ingest_error)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`
	_, err := repo.db.Exec(ctx, query, feedID, guid, bookID, ingestErr)
	return postgres.PgxErrorToHTTPError(err)
}
