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
		SELECT id, email, role FROM global.app_users ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		u.AppAccess = []string{}
		if err = rows.Scan(&u.ID, &u.Email, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *AppUsersRepository) GetByID(
	ctx context.Context,
	id string,
) (*models.User, error) {
	var u models.User
	u.AppAccess = []string{}

	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.role,
		       COALESCE(array_agg(a.app_name) FILTER (WHERE a.app_name IS NOT NULL), '{}')
		FROM global.app_users u
		LEFT JOIN global.app_access a ON a.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id, u.email, u.role
	`, id).Scan(&u.ID, &u.Email, &u.Role, &u.AppAccess)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &u, nil
}

func (r *AppUsersRepository) GetAllWithAccess(
	ctx context.Context,
) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.role,
		       COALESCE(array_agg(a.app_name) FILTER (WHERE a.app_name IS NOT NULL), '{}')
		FROM global.app_users u
		LEFT JOIN global.app_access a ON a.user_id = u.id
		GROUP BY u.id, u.email, u.role
		ORDER BY u.email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		u.AppAccess = []string{}
		if err = rows.Scan(&u.ID, &u.Email, &u.Role, &u.AppAccess); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *AppUsersRepository) SetRole(
	ctx context.Context,
	userID string,
	role models.Role,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE global.app_users SET role = $2 WHERE id = $1`,
		userID, role,
	)
	return err
}

func (r *AppUsersRepository) SetAppAccess(
	ctx context.Context,
	userID, appName string,
	grant bool,
) error {
	var err error
	if grant {
		_, err = r.db.Exec(
			ctx,
			`INSERT INTO global.app_access (user_id, app_name) 
			VALUES ($1, $2) 
			ON CONFLICT DO NOTHING`,
			userID,
			appName,
		)
	} else {
		_, err = r.db.Exec(ctx,
			`DELETE FROM global.app_access WHERE user_id = $1 AND app_name = $2`,
			userID, appName,
		)
	}
	return err
}
