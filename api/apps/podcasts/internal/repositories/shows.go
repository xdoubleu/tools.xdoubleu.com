package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

type ShowsRepository struct {
	db postgres.DB
}

const showColumns = `id, user_id, itunes_id, title, author, artwork_url,
	feed_url, apple_url, added_at, etag, last_modified, fetch_error`

// Add favourites a show; an existing favourite is refreshed from the latest
// iTunes metadata and keeps its added_at.
func (r *ShowsRepository) Add(
	ctx context.Context,
	s models.Show,
) (*models.Show, error) {
	return scanShow(r.db.QueryRow(ctx, `
		INSERT INTO podcasts.shows
			(user_id, itunes_id, title, author, artwork_url, feed_url, apple_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, itunes_id) DO UPDATE SET
			title = EXCLUDED.title,
			author = EXCLUDED.author,
			artwork_url = EXCLUDED.artwork_url,
			-- Validators belong to the old URL.
			etag = CASE WHEN podcasts.shows.feed_url = EXCLUDED.feed_url
				THEN podcasts.shows.etag END,
			last_modified = CASE WHEN podcasts.shows.feed_url = EXCLUDED.feed_url
				THEN podcasts.shows.last_modified END,
			feed_url = EXCLUDED.feed_url,
			apple_url = EXCLUDED.apple_url
		RETURNING `+showColumns,
		s.UserID, s.ITunesID, s.Title, s.Author, s.ArtworkURL, s.FeedURL, s.AppleURL,
	))
}

// List returns the user's favourites, newest first.
func (r *ShowsRepository) List(
	ctx context.Context,
	userID string,
) ([]models.Show, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+showColumns+`
		FROM podcasts.shows
		WHERE user_id = $1
		ORDER BY added_at DESC, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Show{}
	for rows.Next() {
		s, scanErr := scanShow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// Favourited returns which of the given iTunes IDs the user has favourited.
func (r *ShowsRepository) Favourited(
	ctx context.Context,
	userID string,
	itunesIDs []int64,
) (map[int64]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT itunes_id FROM podcasts.shows
		WHERE user_id = $1 AND itunes_id = ANY($2)`, userID, itunesIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Remove deletes one of the user's favourites; another user's reads as
// not found.
func (r *ShowsRepository) Remove(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM podcasts.shows WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// All returns every user's favourites, least recently fetched first.
func (r *ShowsRepository) All(ctx context.Context) ([]models.Show, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+showColumns+`
		FROM podcasts.shows
		ORDER BY last_fetched_at NULLS FIRST, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Show{}
	for rows.Next() {
		s, scanErr := scanShow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// RecordFetch stores a fetch's validators and outcome; an empty fetchError
// means it succeeded.
func (r *ShowsRepository) RecordFetch(
	ctx context.Context,
	id uuid.UUID,
	etag, lastModified *string,
	fetchError string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE podcasts.shows
		SET etag = $2, last_modified = $3, fetch_error = $4,
			last_fetched_at = now()
		WHERE id = $1`, id, etag, lastModified, fetchError)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanShow(row rowScanner) (*models.Show, error) {
	var s models.Show
	err := row.Scan(
		&s.ID, &s.UserID, &s.ITunesID, &s.Title, &s.Author, &s.ArtworkURL,
		&s.FeedURL, &s.AppleURL, &s.AddedAt, &s.ETag, &s.LastModified,
		&s.FetchError,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &s, nil
}
