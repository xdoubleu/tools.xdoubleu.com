package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// FilterRulesRepository stores per-user item filter rules.
type FilterRulesRepository struct {
	db postgres.DB
}

const filterRuleColumns = `r.id, r.user_id, r.feed_id, r.kind, r.value,
	r.created_at`

// filterRuleMatchesItemSQL is matchFilterRule's predicate for rule r and
// item i.
const filterRuleMatchesItemSQL = `CASE r.kind
	WHEN 'category' THEN EXISTS (
	    SELECT 1 FROM unnest(i.categories) AS c (label)
	    WHERE lower(c.label) = lower(r.value)
	)
	ELSE strpos(lower(i.title), lower(r.value)) > 0
END`

func scanFilterRule(row pgx.Row, extra ...any) (*models.FilterRule, error) {
	var r models.FilterRule
	dest := append([]any{
		&r.ID, &r.UserID, &r.FeedID, &r.Kind, &r.Value, &r.CreatedAt,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListForFeed returns the user's rules applying to feedID, its own and the
// global ones.
func (repo *FilterRulesRepository) ListForFeed(
	ctx context.Context,
	userID string,
	feedID uuid.UUID,
) ([]models.FilterRule, error) {
	query := `
		SELECT ` + filterRuleColumns + `
		FROM feeds.filter_rules r
		WHERE r.user_id = $1 AND (r.feed_id IS NULL OR r.feed_id = $2)
		ORDER BY r.created_at, r.id
	`
	rows, err := repo.db.Query(ctx, query, userID, feedID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FilterRule
	for rows.Next() {
		r, scanErr := scanFilterRule(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *r)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// ListByUser returns the user's rules, oldest first, with FilteredCount.
func (repo *FilterRulesRepository) ListByUser(
	ctx context.Context,
	userID string,
) ([]models.FilterRule, error) {
	query := `
		SELECT ` + filterRuleColumns + `,
		    (SELECT count(*) FROM feeds.items i
		     WHERE i.filtered_rule_id = r.id AND i.filtered_at IS NOT NULL)
		FROM feeds.filter_rules r
		WHERE r.user_id = $1
		ORDER BY r.created_at, r.id
	`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.FilterRule
	for rows.Next() {
		var count int
		r, scanErr := scanFilterRule(rows, &count)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		r.FilteredCount = count
		out = append(out, *r)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// Create stores a rule and, atomically, filters the scope's existing items
// that are unread, not bookmarked, not dismissed and not yet filtered.
// FilteredCount is how many it filtered. Returns
// database.ErrResourceNotFound when feedID isn't the user's, and
// database.ErrResourceConflict for a duplicate rule.
func (repo *FilterRulesRepository) Create(
	ctx context.Context,
	userID string,
	feedID *uuid.UUID,
	kind, value string,
) (*models.FilterRule, error) {
	query := `
		WITH r AS (
		    INSERT INTO feeds.filter_rules (user_id, feed_id, kind, value)
		    SELECT $1, $2, $3, $4
		    WHERE $2::uuid IS NULL OR EXISTS (
		        SELECT 1 FROM feeds.feeds WHERE id = $2 AND user_id = $1
		    )
		    RETURNING id, user_id, feed_id, kind, value, created_at
		),
		filtered AS (
		    UPDATE feeds.items i
		    SET filtered_at = now(), filtered_rule_id = r.id
		    FROM r, feeds.feeds f
		    WHERE f.id = i.feed_id AND f.user_id = r.user_id
		      AND (r.feed_id IS NULL OR i.feed_id = r.feed_id)
		      AND i.ingest_error IS NULL AND i.read_at IS NULL
		      AND NOT i.bookmarked AND NOT i.dismissed
		      AND i.filtered_at IS NULL
		      AND ` + filterRuleMatchesItemSQL + `
		    RETURNING i.id
		)
		SELECT ` + filterRuleColumns + `, (SELECT count(*) FROM filtered)
		FROM r
	`
	var count int
	rule, err := scanFilterRule(
		repo.db.QueryRow(ctx, query, userID, feedID, kind, value), &count,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	rule.FilteredCount = count
	return rule, nil
}

// Delete removes one of the user's rules; its items stay filtered with
// filtered_rule_id cleared. Returns database.ErrResourceNotFound when none
// matches.
func (repo *FilterRulesRepository) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	tag, err := repo.db.Exec(ctx,
		`DELETE FROM feeds.filter_rules WHERE user_id = $1 AND id = $2`,
		userID, id,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}
