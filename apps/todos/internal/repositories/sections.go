package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type SectionsRepository struct {
	db postgres.DB
}

func (r *SectionsRepository) ListByUser(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.Section, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_user_id, name, sort_order, created_at
		FROM todos.sections
		WHERE owner_user_id = $1 AND workspace_id IS NOT DISTINCT FROM $2
		ORDER BY sort_order, created_at`,
		userID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSections(rows)
}

func (r *SectionsRepository) Create(
	ctx context.Context,
	s models.Section,
) (*models.Section, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO todos.sections (owner_user_id, name, sort_order, workspace_id)
		VALUES ($1, $2,
		    COALESCE((
		        SELECT MAX(sort_order) + 1
		        FROM todos.sections
		        WHERE owner_user_id = $1 AND workspace_id IS NOT DISTINCT FROM $3
		    ), 0), $3)
		RETURNING id, sort_order, created_at`,
		s.OwnerUserID, s.Name, s.WorkspaceID,
	).Scan(&s.ID, &s.SortOrder, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SectionsRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.sections WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	)
	return err
}

func scanSections(rows pgx.Rows) ([]models.Section, error) {
	var result []models.Section
	for rows.Next() {
		var s models.Section
		if err := rows.Scan(
			&s.ID, &s.OwnerUserID, &s.Name, &s.SortOrder, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
