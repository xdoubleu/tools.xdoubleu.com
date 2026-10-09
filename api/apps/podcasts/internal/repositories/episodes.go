package repositories

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type EpisodesRepository struct {
	db postgres.DB
}

// episodeColumns is the number of columns an episode's INSERT row binds.
const episodeColumns = 8

// Upsert stores a show's episodes in one statement; one already stored is
// refreshed, since publishers fix titles and links after the fact.
func (r *EpisodesRepository) Upsert(
	ctx context.Context,
	showID uuid.UUID,
	episodes []models.Episode,
) error {
	episodes = dedupeByGUID(episodes)
	if len(episodes) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(`
		INSERT INTO podcasts.episodes
			(show_id, guid, title, summary, link, audio_url,
			 duration_seconds, published_at)
		VALUES`)
	args := make([]any, 0, len(episodes)*episodeColumns)
	for i, e := range episodes {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(" (")
		for j := range episodeColumns {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "$%d", i*episodeColumns+j+1)
		}
		b.WriteString(")")
		args = append(args, showID, e.GUID, e.Title, e.Summary, e.Link, e.AudioURL,
			e.DurationSeconds, e.PublishedAt)
	}
	b.WriteString(`
		ON CONFLICT (show_id, guid) DO UPDATE SET
			title = EXCLUDED.title,
			summary = EXCLUDED.summary,
			link = EXCLUDED.link,
			audio_url = EXCLUDED.audio_url,
			duration_seconds = EXCLUDED.duration_seconds,
			published_at = EXCLUDED.published_at`)

	_, err := r.db.Exec(ctx, b.String(), args...)
	return err
}

// dedupeByGUID keeps the last episode per GUID, matching the loop's "last
// upsert wins"; a repeated GUID in one batch would otherwise fail the
// multi-row upsert with "cannot affect row a second time".
func dedupeByGUID(episodes []models.Episode) []models.Episode {
	seen := make(map[string]struct{}, len(episodes))
	uniq := make([]models.Episode, 0, len(episodes))
	for i := len(episodes) - 1; i >= 0; i-- {
		if _, ok := seen[episodes[i].GUID]; ok {
			continue
		}
		seen[episodes[i].GUID] = struct{}{}
		uniq = append(uniq, episodes[i])
	}
	slices.Reverse(uniq)
	return uniq
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
