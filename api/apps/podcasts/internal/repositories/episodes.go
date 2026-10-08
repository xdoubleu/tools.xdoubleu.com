package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type EpisodesRepository struct {
	db postgres.DB
}

// Upsert stores a show's episodes; one already stored is refreshed, since
// publishers fix titles and links after the fact.
func (r *EpisodesRepository) Upsert(
	ctx context.Context,
	showID uuid.UUID,
	episodes []models.Episode,
) error {
	for _, e := range episodes {
		_, err := r.db.Exec(ctx, `
			INSERT INTO podcasts.episodes
				(show_id, guid, title, summary, link, audio_url,
				 duration_seconds, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (show_id, guid) DO UPDATE SET
				title = EXCLUDED.title,
				summary = EXCLUDED.summary,
				link = EXCLUDED.link,
				audio_url = EXCLUDED.audio_url,
				duration_seconds = EXCLUDED.duration_seconds,
				published_at = EXCLUDED.published_at`,
			showID, e.GUID, e.Title, e.Summary, e.Link, e.AudioURL,
			e.DurationSeconds, e.PublishedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// List returns the user's episodes newest first, optionally of one show;
// limit is the SQL LIMIT (callers ask for one extra to detect more).
func (r *EpisodesRepository) List(
	ctx context.Context,
	userID string,
	showID *uuid.UUID,
	limit, offset int,
) ([]models.ListedEpisode, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.show_id, e.guid, e.title, e.summary, e.link, e.audio_url,
			e.duration_seconds, e.published_at,
			s.title, s.artwork_url, s.apple_url
		FROM podcasts.episodes e
		JOIN podcasts.shows s ON s.id = e.show_id
		WHERE s.user_id = $1 AND ($2::uuid IS NULL OR s.id = $2)
		ORDER BY e.published_at DESC NULLS LAST, e.id
		LIMIT $3 OFFSET $4`, userID, showID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.ListedEpisode{}
	for rows.Next() {
		var e models.ListedEpisode
		err = rows.Scan(
			&e.ID, &e.ShowID, &e.GUID, &e.Title, &e.Summary, &e.Link, &e.AudioURL,
			&e.DurationSeconds, &e.PublishedAt,
			&e.ShowTitle, &e.ArtworkURL, &e.AppleURL,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
