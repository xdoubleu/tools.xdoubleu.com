package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type WorkspacesRepository struct {
	db postgres.DB
}

func (r *WorkspacesRepository) ListByUser(
	ctx context.Context,
	userID string,
) ([]models.Workspace, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_user_id, name, created_at
		FROM todos.workspaces
		WHERE owner_user_id = $1
		ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkspaces(rows)
}

func (r *WorkspacesRepository) Create(
	ctx context.Context,
	w models.Workspace,
) (*models.Workspace, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO todos.workspaces (owner_user_id, name)
		VALUES ($1, $2)
		RETURNING id, created_at`,
		w.OwnerUserID, w.Name,
	).Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkspacesRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.workspaces WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	)
	return err
}

func scanWorkspaces(rows pgx.Rows) ([]models.Workspace, error) {
	var result []models.Workspace
	for rows.Next() {
		var w models.Workspace
		if err := rows.Scan(&w.ID, &w.OwnerUserID, &w.Name, &w.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
