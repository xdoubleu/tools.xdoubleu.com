package repositories

import (
	"context"

	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type AppUsersRepository struct {
	db postgres.DB
}

func NewAppUsersRepository(db postgres.DB) *AppUsersRepository {
	return &AppUsersRepository{db: db}
}

func (r *AppUsersRepository) Upsert(ctx context.Context, id, email string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.app_users (id, email, last_seen)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET
			email     = EXCLUDED.email,
			last_seen = now()
	`, id, email)
	return err
}

func (r *AppUsersRepository) GetAll(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email FROM global.app_users ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err = rows.Scan(&u.ID, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
