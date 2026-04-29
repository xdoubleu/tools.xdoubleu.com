package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type SettingsRepository struct {
	db postgres.DB
}

func (r *SettingsRepository) GetLabelPresets(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) (*models.LabelPresets, error) {
	rows, err := r.db.Query(ctx, `
		SELECT category, value
		FROM todos.label_presets
		WHERE user_id = $1 AND workspace_id IS NOT DISTINCT FROM $2
		ORDER BY category, value`,
		userID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	presets := &models.LabelPresets{
		Setups: []string{},
		Types:  []string{},
	}
	for rows.Next() {
		var category, value string
		if err = rows.Scan(&category, &value); err != nil {
			return nil, err
		}
		switch category {
		case models.LabelCategorySetup:
			presets.Setups = append(presets.Setups, value)
		case models.LabelCategoryType:
			presets.Types = append(presets.Types, value)
		}
	}
	return presets, rows.Err()
}

func (r *SettingsRepository) AddLabelPreset(
	ctx context.Context,
	userID string,
	category string,
	value string,
	workspaceID *uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.label_presets (user_id, category, value, sort_order, workspace_id)
		VALUES ($1, $2, $3,
		    COALESCE((
		        SELECT MAX(sort_order) + 1
		        FROM todos.label_presets
		        WHERE user_id = $1 AND category = $2
		            AND workspace_id IS NOT DISTINCT FROM $4
		    ), 0), $4)
		ON CONFLICT ON CONSTRAINT label_presets_unique DO NOTHING`,
		userID, category, value, workspaceID,
	)
	return err
}

func (r *SettingsRepository) RemoveLabelPreset(
	ctx context.Context,
	userID string,
	category string,
	value string,
	workspaceID *uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM todos.label_presets
		WHERE user_id = $1 AND category = $2 AND value = $3
		    AND workspace_id IS NOT DISTINCT FROM $4`,
		userID, category, value, workspaceID,
	)
	return err
}

func (r *SettingsRepository) GetURLPatterns(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.URLPattern, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, url_prefix, platform_name, type_label, sort_order
		FROM todos.url_patterns
		WHERE user_id = $1 AND workspace_id IS NOT DISTINCT FROM $2
		ORDER BY sort_order, platform_name`,
		userID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanURLPatterns(rows)
}

func (r *SettingsRepository) AddURLPattern(
	ctx context.Context,
	p models.URLPattern,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.url_patterns
		    (user_id, url_prefix, platform_name, type_label, sort_order, workspace_id)
		VALUES ($1, $2, $3, $4,
		    COALESCE((
		        SELECT MAX(sort_order) + 1
		        FROM todos.url_patterns
		        WHERE user_id = $1 AND workspace_id IS NOT DISTINCT FROM $5
		    ), 0), $5)`,
		p.UserID, p.URLPrefix, p.PlatformName, p.TypeLabel, p.WorkspaceID,
	)
	return err
}

func (r *SettingsRepository) RemoveURLPattern(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.url_patterns WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

func (r *SettingsRepository) GetArchiveSettings(
	ctx context.Context,
	userID string,
) (*models.ArchiveSettings, error) {
	// Upsert a default row so callers always get a result.
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.archive_settings (user_id, archive_after_hours)
		VALUES ($1, 24) ON CONFLICT DO NOTHING`,
		userID,
	)
	if err != nil {
		return nil, err
	}

	s := &models.ArchiveSettings{
		UserID:            userID,
		ArchiveAfterHours: 0,
	}
	err = r.db.QueryRow(ctx,
		`SELECT archive_after_hours FROM todos.archive_settings WHERE user_id = $1`,
		userID,
	).Scan(&s.ArchiveAfterHours)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return s, nil
}

func (r *SettingsRepository) UpsertArchiveSettings(
	ctx context.Context,
	s models.ArchiveSettings,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.archive_settings (user_id, archive_after_hours)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET archive_after_hours = EXCLUDED.archive_after_hours`,
		s.UserID, s.ArchiveAfterHours,
	)
	return err
}

func (r *SettingsRepository) GetUserSettings(
	ctx context.Context,
	userID string,
) (*models.UserSettings, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO todos.user_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	if err != nil {
		return nil, err
	}

	s := &models.UserSettings{
		UserID:            userID,
		ActiveWorkspaceID: nil,
		ActiveWorkspace:   nil,
	}
	var wsID *uuid.UUID
	var wsName *string
	var wsCreatedAt *string
	err = r.db.QueryRow(ctx, `
		SELECT us.active_workspace_id, w.name, w.created_at::text
		FROM todos.user_settings us
		LEFT JOIN todos.workspaces w ON w.id = us.active_workspace_id
		WHERE us.user_id = $1`,
		userID,
	).Scan(&wsID, &wsName, &wsCreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	s.ActiveWorkspaceID = wsID
	if wsID != nil && wsName != nil {
		s.ActiveWorkspace = &models.Workspace{
			ID:          *wsID,
			OwnerUserID: userID,
			Name:        *wsName,
			CreatedAt:   time.Time{},
		}
	}
	return s, nil
}

func (r *SettingsRepository) SetActiveWorkspace(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.user_settings (user_id, active_workspace_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET active_workspace_id = EXCLUDED.active_workspace_id`,
		userID, workspaceID,
	)
	return err
}

func scanURLPatterns(rows pgx.Rows) ([]models.URLPattern, error) {
	var result []models.URLPattern
	for rows.Next() {
		var p models.URLPattern
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.URLPrefix, &p.PlatformName, &p.TypeLabel, &p.SortOrder,
		); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
