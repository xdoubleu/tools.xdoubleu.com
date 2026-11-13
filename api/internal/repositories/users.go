package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/kit/database/postgres"
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
		SELECT id, email, role FROM global.app_users
		WHERE role NOT IN ('service', 'agent')
		ORDER BY last_seen DESC
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
	u.WriteAccess = []string{}

	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.role, COALESCE(u.display_name, ''),
		       COALESCE(array_agg(a.app_name) FILTER (WHERE a.app_name IS NOT NULL), '{}'),
		       COALESCE(array_agg(a.app_name) FILTER (WHERE a.level = 'write'), '{}')
		FROM global.app_users u
		LEFT JOIN global.app_access a ON a.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id, u.email, u.role, u.display_name
	`, id).Scan(
		&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AppAccess, &u.WriteAccess,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &u, nil
}

// SetDisplayName sets the user's public profile display name.
func (r *AppUsersRepository) SetDisplayName(
	ctx context.Context,
	userID, displayName string,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE global.app_users SET display_name = $2 WHERE id = $1`,
		userID, displayName,
	)
	return err
}

// GetTabPins returns the apps pinned to the phone tab bar, in order. The bool is
// false when the user never chose, so the web falls back to its defaults.
func (r *AppUsersRepository) GetTabPins(
	ctx context.Context,
	userID string,
) ([]string, bool, error) {
	var pins []string
	err := r.db.QueryRow(ctx,
		`SELECT tab_pins FROM global.app_users WHERE id = $1`, userID,
	).Scan(&pins)
	if err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}
	return pins, pins != nil, nil
}

// SetTabPins stores the apps pinned to the phone tab bar, replacing any earlier
// choice. An empty (non-nil) list is a deliberate "none".
func (r *AppUsersRepository) SetTabPins(
	ctx context.Context,
	userID string,
	pins []string,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE global.app_users SET tab_pins = $2 WHERE id = $1`,
		userID, pins,
	)
	return err
}

const (
	appOpenRetentionDays = 90
	suggestedPinWindow   = 30
	suggestedPinLimit    = 8
)

// RecordAppOpen counts one open of app for the user today and drops that user's
// counts older than 90 days.
func (r *AppUsersRepository) RecordAppOpen(
	ctx context.Context,
	userID, app string,
) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO global.app_opens (user_id, day, app, count)
		VALUES ($1, CURRENT_DATE, $2, 1)
		ON CONFLICT (user_id, day, app)
		DO UPDATE SET count = app_opens.count + 1
	`, userID, app); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM global.app_opens
		WHERE user_id = $1 AND day < CURRENT_DATE - $2::int
	`, userID, appOpenRetentionDays)
	return err
}

// MostOpenedApps ranks the apps the user opened over the last 30 whole days,
// most first. Today is left out so the ranking holds still for a day.
func (r *AppUsersRepository) MostOpenedApps(
	ctx context.Context,
	userID string,
) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT app
		FROM global.app_opens
		WHERE user_id = $1
		  AND day >= CURRENT_DATE - $2::int AND day < CURRENT_DATE
		GROUP BY app
		ORDER BY SUM(count) DESC, app
		LIMIT $3
	`, userID, suggestedPinWindow, suggestedPinLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := []string{}
	for rows.Next() {
		var app string
		if err = rows.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (r *AppUsersRepository) GetAllWithAccess(
	ctx context.Context,
) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.role, COALESCE(u.display_name, ''),
		       COALESCE(array_agg(a.app_name) FILTER (WHERE a.app_name IS NOT NULL), '{}')
		FROM global.app_users u
		LEFT JOIN global.app_access a ON a.user_id = u.id
		WHERE u.role <> 'agent'
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
		err = rows.Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AppAccess)
		if err != nil {
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
		`UPDATE global.app_users SET role = $2 WHERE id = $1 AND role <> 'agent'`,
		userID, role,
	)
	return err
}

// Agent access levels; AgentNoAccess revokes the grant (adr-0047).
const (
	AgentNoAccess = "none"
	AgentRead     = "read"
	AgentWrite    = "write"
)

// SetAgentAppAccess grants a standing agent an app at AgentRead or AgentWrite,
// or revokes it with AgentNoAccess. A person, a job's own account and a revoked
// account hold no agent grants: the bool is false, and nothing changes, when
// userID is not an active standing agent. The account row is locked for the
// write, so it can't race a revoke.
func (r *AppUsersRepository) SetAgentAppAccess(
	ctx context.Context,
	userID, appName, level string,
) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var found bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM global.agent_accounts
			WHERE user_id::text = $1 AND job_id IS NULL AND revoked_at IS NULL
			FOR UPDATE
		)`, userID).Scan(&found)
	if err != nil || !found {
		return false, err
	}

	if level == AgentNoAccess {
		_, err = tx.Exec(ctx,
			`DELETE FROM global.app_access WHERE user_id = $1 AND app_name = $2`,
			userID, appName,
		)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO global.app_access (user_id, app_name, level)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, app_name) DO UPDATE SET level = EXCLUDED.level`,
			userID, appName, level,
		)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
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
			SELECT id, $2 FROM global.app_users
			WHERE id = $1 AND role <> 'agent'
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
