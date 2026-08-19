package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
)

// User is the row shape read back from auth.users.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// TOTPFactor is a row from auth.totp_factors.
type TOTPFactor struct {
	ID     uuid.UUID
	UserID string
	Secret string
	Status string
}

// RecoveryCode is a row from auth.recovery_codes.
type RecoveryCode struct {
	ID       uuid.UUID
	UserID   string
	CodeHash string
}

// RefreshTokenRow is a row from auth.refresh_tokens.
type RefreshTokenRow struct {
	ID        uuid.UUID
	UserID    string
	AAL       string
	ExpiresAt time.Time
}

// PasswordResetTokenRow is a row from auth.password_reset_tokens.
type PasswordResetTokenRow struct {
	ID        uuid.UUID
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// usersStore is the subset of DB access LocalService needs, narrowed to an
// interface (mirroring appUsersStore) so tests can fake individual failures.
type usersStore interface {
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
	SetPasswordHash(ctx context.Context, userID, hash string) error

	CreateTOTPFactor(
		ctx context.Context, userID, secret string,
	) (uuid.UUID, error)
	GetVerifiedTOTPFactor(
		ctx context.Context, userID string,
	) (*TOTPFactor, error)
	GetTOTPFactor(ctx context.Context, factorID uuid.UUID) (*TOTPFactor, error)
	VerifyTOTPFactor(ctx context.Context, factorID uuid.UUID) error
	DeleteUnverifiedTOTPFactors(ctx context.Context, userID string) error
	DeleteAllTOTPFactors(ctx context.Context, userID string) error

	CreateRecoveryCodes(
		ctx context.Context, userID string, hashes []string,
	) error
	GetUnusedRecoveryCodes(
		ctx context.Context, userID string,
	) ([]RecoveryCode, error)
	MarkRecoveryCodeUsed(ctx context.Context, id uuid.UUID) error
	DeleteRecoveryCodes(ctx context.Context, userID string) error

	CreateRefreshToken(
		ctx context.Context, userID, tokenHash, aal string, expiresAt time.Time,
	) error
	GetRefreshTokenByHash(
		ctx context.Context, tokenHash string,
	) (*RefreshTokenRow, error)
	DeleteRefreshToken(ctx context.Context, id uuid.UUID) error
	DeleteAllRefreshTokensForUser(ctx context.Context, userID string) error

	CreatePasswordResetToken(
		ctx context.Context, userID, tokenHash string, expiresAt time.Time,
	) error
	GetPasswordResetTokenByHash(
		ctx context.Context, tokenHash string,
	) (*PasswordResetTokenRow, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error
}

// Repository is the Postgres-backed implementation of usersStore, over the
// `auth` schema (issue #1039).
type Repository struct {
	db postgres.DB
}

func NewRepository(db postgres.DB) *Repository {
	return &Repository{db: db}
}

var _ usersStore = (*Repository)(nil)

// errNotFound is a sentinel wrapping pgx.ErrNoRows lookups so callers can
// treat "user/token doesn't exist" uniformly.
var errNotFound = errors.New("auth: not found")

func wrapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	return err
}

func (r *Repository) GetUserByEmail(
	ctx context.Context,
	email string,
) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM auth.users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(
	ctx context.Context,
	id string,
) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, created_at
		FROM auth.users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &u, nil
}

func (r *Repository) CreateUser(
	ctx context.Context,
	email, passwordHash string,
) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at
	`, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) SetPasswordHash(
	ctx context.Context,
	userID, hash string,
) error {
	_, err := r.db.Exec(
		ctx, `UPDATE auth.users SET password_hash = $2 WHERE id = $1`,
		userID, hash,
	)
	return err
}

func (r *Repository) CreateTOTPFactor(
	ctx context.Context,
	userID, secret string,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth.totp_factors (user_id, secret, status)
		VALUES ($1, $2, 'unverified')
		RETURNING id
	`, userID, secret).Scan(&id)
	return id, err
}

func (r *Repository) GetVerifiedTOTPFactor(
	ctx context.Context,
	userID string,
) (*TOTPFactor, error) {
	var f TOTPFactor
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, secret, status FROM auth.totp_factors
		WHERE user_id = $1 AND status = 'verified'
	`, userID).Scan(&f.ID, &f.UserID, &f.Secret, &f.Status)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &f, nil
}

func (r *Repository) GetTOTPFactor(
	ctx context.Context,
	factorID uuid.UUID,
) (*TOTPFactor, error) {
	var f TOTPFactor
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, secret, status FROM auth.totp_factors
		WHERE id = $1
	`, factorID).Scan(&f.ID, &f.UserID, &f.Secret, &f.Status)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &f, nil
}

func (r *Repository) VerifyTOTPFactor(
	ctx context.Context,
	factorID uuid.UUID,
) error {
	_, err := r.db.Exec(
		ctx, `UPDATE auth.totp_factors SET status = 'verified' WHERE id = $1`,
		factorID,
	)
	return err
}

func (r *Repository) DeleteUnverifiedTOTPFactors(
	ctx context.Context,
	userID string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM auth.totp_factors WHERE user_id = $1 AND status = 'unverified'
	`, userID)
	return err
}

func (r *Repository) DeleteAllTOTPFactors(
	ctx context.Context,
	userID string,
) error {
	_, err := r.db.Exec(
		ctx, `DELETE FROM auth.totp_factors WHERE user_id = $1`, userID,
	)
	return err
}

func (r *Repository) CreateRecoveryCodes(
	ctx context.Context,
	userID string,
	hashes []string,
) error {
	//nolint:exhaustruct //QueuedQueries is populated by Queue below
	batch := &pgx.Batch{}
	for _, h := range hashes {
		batch.Queue(
			`INSERT INTO auth.recovery_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, h,
		)
	}
	results := r.db.SendBatch(ctx, batch)
	defer results.Close()

	for range hashes {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return results.Close()
}

func (r *Repository) GetUnusedRecoveryCodes(
	ctx context.Context,
	userID string,
) ([]RecoveryCode, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, code_hash FROM auth.recovery_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []RecoveryCode
	for rows.Next() {
		var c RecoveryCode
		if err = rows.Scan(&c.ID, &c.UserID, &c.CodeHash); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

func (r *Repository) MarkRecoveryCodeUsed(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := r.db.Exec(
		ctx, `UPDATE auth.recovery_codes SET used_at = now() WHERE id = $1`, id,
	)
	return err
}

func (r *Repository) DeleteRecoveryCodes(
	ctx context.Context,
	userID string,
) error {
	_, err := r.db.Exec(
		ctx, `DELETE FROM auth.recovery_codes WHERE user_id = $1`, userID,
	)
	return err
}

func (r *Repository) CreateRefreshToken(
	ctx context.Context,
	userID, tokenHash, aal string,
	expiresAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.refresh_tokens (user_id, token_hash, aal, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, tokenHash, aal, expiresAt)
	return err
}

func (r *Repository) GetRefreshTokenByHash(
	ctx context.Context,
	tokenHash string,
) (*RefreshTokenRow, error) {
	var t RefreshTokenRow
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, aal, expires_at FROM auth.refresh_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.AAL, &t.ExpiresAt)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &t, nil
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth.refresh_tokens WHERE id = $1`, id)
	return err
}

func (r *Repository) DeleteAllRefreshTokensForUser(
	ctx context.Context,
	userID string,
) error {
	_, err := r.db.Exec(
		ctx, `DELETE FROM auth.refresh_tokens WHERE user_id = $1`, userID,
	)
	return err
}

func (r *Repository) CreatePasswordResetToken(
	ctx context.Context,
	userID, tokenHash string,
	expiresAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (r *Repository) GetPasswordResetTokenByHash(
	ctx context.Context,
	tokenHash string,
) (*PasswordResetTokenRow, error) {
	var t PasswordResetTokenRow
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at FROM auth.password_reset_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &t, nil
}

func (r *Repository) MarkPasswordResetTokenUsed(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.password_reset_tokens SET used_at = now() WHERE id = $1
	`, id)
	return err
}
