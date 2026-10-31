package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type ReposRepository struct {
	db postgres.DB
}

const repoColumns = `id, owner, name, default_branch, default_harness,
	setup_commands, agent_instructions, time_limit_minutes, created_at,
	updated_at`

func scanRepo(row rowScanner) (*models.Repo, error) {
	var r models.Repo
	err := row.Scan(
		&r.ID, &r.Owner, &r.Name, &r.DefaultBranch, &r.DefaultHarness,
		&r.SetupCommands, &r.AgentInstructions, &r.TimeLimitMinutes,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &r, nil
}

// List returns every repo, ordered by owner and name.
func (r *ReposRepository) List(ctx context.Context) ([]models.Repo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+repoColumns+` FROM factory.repos ORDER BY owner, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Repo{}
	for rows.Next() {
		repo, scanErr := scanRepo(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *repo)
	}
	return out, rows.Err()
}

// Create inserts a repo; a repeated owner/name is a conflict.
func (r *ReposRepository) Create(
	ctx context.Context,
	repo models.Repo,
) (*models.Repo, error) {
	return scanRepo(r.db.QueryRow(ctx, `
		INSERT INTO factory.repos
			(owner, name, default_branch, default_harness, setup_commands,
			 agent_instructions, time_limit_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+repoColumns,
		repo.Owner, repo.Name, repo.DefaultBranch, repo.DefaultHarness,
		repo.SetupCommands, repo.AgentInstructions, repo.TimeLimitMinutes,
	))
}

// Update replaces every field except the id.
func (r *ReposRepository) Update(
	ctx context.Context,
	repo models.Repo,
) (*models.Repo, error) {
	return scanRepo(r.db.QueryRow(ctx, `
		UPDATE factory.repos SET
			owner = $2, name = $3, default_branch = $4, default_harness = $5,
			setup_commands = $6, agent_instructions = $7,
			time_limit_minutes = $8, updated_at = now()
		WHERE id = $1
		RETURNING `+repoColumns,
		repo.ID, repo.Owner, repo.Name, repo.DefaultBranch, repo.DefaultHarness,
		repo.SetupCommands, repo.AgentInstructions, repo.TimeLimitMinutes,
	))
}

// Get returns one repo.
func (r *ReposRepository) Get(
	ctx context.Context,
	id uuid.UUID,
) (*models.Repo, error) {
	return scanRepo(r.db.QueryRow(ctx, `
		SELECT `+repoColumns+` FROM factory.repos WHERE id = $1`, id))
}
