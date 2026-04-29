package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type PoliciesRepository struct {
	db postgres.DB
}

func (r *PoliciesRepository) ListByUser(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.Policy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_user_id, text, reappear_after_hours, sort_order, created_at
		FROM todos.policies
		WHERE owner_user_id = $1 AND workspace_id IS NOT DISTINCT FROM $2
		ORDER BY sort_order, created_at`,
		userID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolicies(rows)
}

func (r *PoliciesRepository) Create(
	ctx context.Context,
	p models.Policy,
) (*models.Policy, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO todos.policies
		    (owner_user_id, text, reappear_after_hours, sort_order, workspace_id)
		VALUES ($1, $2, $3,
		    COALESCE((
		        SELECT MAX(sort_order) + 1
		        FROM todos.policies
		        WHERE owner_user_id = $1 AND workspace_id IS NOT DISTINCT FROM $4
		    ), 0), $4)
		RETURNING id, sort_order, created_at`,
		p.OwnerUserID, p.Text, p.ReappearAfterHours, p.WorkspaceID,
	).Scan(&p.ID, &p.SortOrder, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PoliciesRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.policies WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	)
	return err
}

func scanPolicies(rows pgx.Rows) ([]models.Policy, error) {
	var result []models.Policy
	for rows.Next() {
		var p models.Policy
		if err := rows.Scan(
			&p.ID, &p.OwnerUserID, &p.Text,
			&p.ReappearAfterHours, &p.SortOrder, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
