// Package agentaccounts manages agent accounts: a user with the agent role
// plus a confidential client_credentials OAuth client, so an agent acts on the
// factory board under its own identity, never as a person (adr-0039).
package agentaccounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// secretBytes of randomness make a 43-character URL-safe client secret.
const (
	secretBytes   = 32
	clientIDBytes = 16
	// clientSecretBCryptCost matches fosite's DefaultBCryptWorkFactor.
	clientSecretBCryptCost = 12
	// unusablePasswordHash can't match any password, so the user can't sign in.
	unusablePasswordHash = "!"
	clientIDPrefix       = "agent-"
)

// Account is an agent account. RevokedAt is nil while it is active. A job's own
// account has JobID and TaskID set and may act on that task only.
type Account struct {
	ID        uuid.UUID
	Name      string
	UserID    uuid.UUID
	ClientID  string
	CreatedBy string
	CreatedAt time.Time
	RevokedAt *time.Time
	JobID     *uuid.UUID
	TaskID    *uuid.UUID
}

// Scope is what a job's account is confined to.
type Scope struct {
	JobID  uuid.UUID
	TaskID uuid.UUID
}

// Store persists agent accounts in the global and auth schemas.
type Store struct {
	db postgres.DB
	// onRevoke runs after a revoke, to drop cached sessions of the user.
	onRevoke func()
}

// NewStore builds a Store; onRevoke may be nil.
func NewStore(db postgres.DB, onRevoke func()) *Store {
	return &Store{db: db, onRevoke: onRevoke}
}

func randomString(n int, encode func([]byte) string) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return encode(buf), nil
}

const accountColumns = `id, name, user_id, client_id, created_by, created_at,
	revoked_at, job_id, task_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (*Account, error) {
	var a Account
	err := row.Scan(
		&a.ID, &a.Name, &a.UserID, &a.ClientID, &a.CreatedBy, &a.CreatedAt,
		&a.RevokedAt, &a.JobID, &a.TaskID,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &a, nil
}

// Create adds the user, client and account row in one transaction and returns
// the client secret, which is not stored in the clear and can't be shown again.
func (s *Store) Create(
	ctx context.Context,
	createdBy, name string,
) (*Account, string, error) {
	return s.create(ctx, createdBy, name, nil)
}

// CreateForJob is Create for a job's own account, confined to the job's task.
func (s *Store) CreateForJob(
	ctx context.Context,
	createdBy, name string,
	scope Scope,
) (*Account, string, error) {
	return s.create(ctx, createdBy, name, &scope)
}

func (s *Store) create(
	ctx context.Context,
	createdBy, name string,
	scope *Scope,
) (*Account, string, error) {
	secret, err := randomString(secretBytes, base64.RawURLEncoding.EncodeToString)
	if err != nil {
		return nil, "", err
	}
	suffix, err := randomString(clientIDBytes, hex.EncodeToString)
	if err != nil {
		return nil, "", err
	}
	clientID := clientIDPrefix + suffix
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), clientSecretBCryptCost)
	if err != nil {
		return nil, "", err
	}

	userID := uuid.New()
	email := "agent-" + userID.String() + "@agent.invalid"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx,
		`INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)`,
		userID, email, unusablePasswordHash,
	); err != nil {
		return nil, "", err
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO global.app_users (id, email, role, display_name)
		 VALUES ($1, $2, 'agent', $3)`,
		userID, email, name,
	); err != nil {
		return nil, "", err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO auth.oauth2_clients
			(id, secret_hash, grant_types, public, client_name)
		VALUES ($1, $2, ARRAY['client_credentials'], FALSE, $3)`,
		clientID, string(hash), name,
	); err != nil {
		return nil, "", err
	}
	var jobID, taskID *uuid.UUID
	if scope != nil {
		jobID, taskID = &scope.JobID, &scope.TaskID
	}
	account, err := scanAccount(tx.QueryRow(ctx, `
		INSERT INTO global.agent_accounts
			(user_id, client_id, name, created_by, job_id, task_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+accountColumns,
		userID, clientID, name, createdBy, jobID, taskID,
	))
	if err != nil {
		return nil, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return account, secret, nil
}

// List returns the accounts a person made, newest first, revoked ones included.
// A job's own accounts come and go with their jobs and stay out of it.
func (s *Store) List(ctx context.Context) ([]Account, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+accountColumns+`
		FROM global.agent_accounts
		WHERE job_id IS NULL
		ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Account{}
	for rows.Next() {
		a, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// Revoke ends an account: it stamps revoked_at and deletes the client, whose
// cascade removes the tokens it issued, so they stop working at once. Revoking
// a revoked account does nothing; an unknown id is not found.
func (s *Store) Revoke(ctx context.Context, id uuid.UUID) (*Account, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	account, err := scanAccount(tx.QueryRow(ctx, `
		SELECT `+accountColumns+` FROM global.agent_accounts
		WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if account.RevokedAt == nil {
		account, err = scanAccount(tx.QueryRow(ctx, `
			UPDATE global.agent_accounts SET revoked_at = now()
			WHERE id = $1
			RETURNING `+accountColumns, id))
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx,
			`DELETE FROM auth.oauth2_clients WHERE id = $1`, account.ClientID,
		); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.onRevoke != nil {
		s.onRevoke()
	}
	return account, nil
}

// ClientSubject returns the user an active account's client acts as; a client
// that isn't an active agent account has none.
func (s *Store) ClientSubject(
	ctx context.Context,
	clientID string,
) (string, bool, error) {
	var userID string
	err := s.db.QueryRow(ctx, `
		SELECT user_id::text FROM global.agent_accounts
		WHERE client_id = $1 AND revoked_at IS NULL`, clientID,
	).Scan(&userID)
	if errors.Is(postgres.PgxErrorToHTTPError(err), database.ErrResourceNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return userID, true, nil
}

// RevokeForJob ends the job's account, if it has one and it is still active.
func (s *Store) RevokeForJob(ctx context.Context, jobID uuid.UUID) error {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id FROM global.agent_accounts
		WHERE job_id = $1 AND revoked_at IS NULL`, jobID).Scan(&id)
	if errors.Is(postgres.PgxErrorToHTTPError(err), database.ErrResourceNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.Revoke(ctx, id)
	return err
}

// ActiveJobs lists the jobs that still have an active account.
func (s *Store) ActiveJobs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `
		SELECT job_id FROM global.agent_accounts
		WHERE job_id IS NOT NULL AND revoked_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ScopeOf is what the user's account is confined to; an account made by a person
// (or no account) has none.
func (s *Store) ScopeOf(ctx context.Context, userID uuid.UUID) (*Scope, error) {
	var scope Scope
	err := s.db.QueryRow(ctx, `
		SELECT job_id, task_id FROM global.agent_accounts
		WHERE user_id = $1 AND job_id IS NOT NULL AND revoked_at IS NULL`, userID,
	).Scan(&scope.JobID, &scope.TaskID)
	if errors.Is(postgres.PgxErrorToHTTPError(err), database.ErrResourceNotFound) {
		return nil, nil //nolint:nilnil // no scope is an answer, not an error
	}
	if err != nil {
		return nil, err
	}
	return &scope, nil
}
