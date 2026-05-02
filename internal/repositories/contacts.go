package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type ContactsRepository struct {
	db postgres.DB
}

func NewContactsRepository(db postgres.DB) *ContactsRepository {
	return &ContactsRepository{db: db}
}

func (r *ContactsRepository) scan(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}) ([]models.Contact, error) {
	defer rows.Close()
	var result []models.Contact
	for rows.Next() {
		var c models.Contact
		if err := rows.Scan(
			&c.ID, &c.OwnerUserID, &c.ContactUserID,
			&c.DisplayName, &c.Status, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

const selectContacts = `
	SELECT id, owner_user_id, contact_user_id, display_name, status, created_at
	FROM global.contacts`

// List returns accepted contacts for the given user.
func (r *ContactsRepository) List(
	ctx context.Context,
	ownerUserID string,
) ([]models.Contact, error) {
	rows, err := r.db.Query(ctx,
		selectContacts+` WHERE owner_user_id = $1 AND status = 'accepted'
		ORDER BY display_name`,
		ownerUserID,
	)
	if err != nil {
		return nil, err
	}
	return r.scan(rows)
}

// ListPending returns sent contact requests not yet accepted.
func (r *ContactsRepository) ListPending(
	ctx context.Context,
	ownerUserID string,
) ([]models.Contact, error) {
	rows, err := r.db.Query(ctx,
		selectContacts+` WHERE owner_user_id = $1 AND status = 'pending'
		ORDER BY display_name`,
		ownerUserID,
	)
	if err != nil {
		return nil, err
	}
	return r.scan(rows)
}

// ListIncoming returns pending contact requests addressed to userID.
func (r *ContactsRepository) ListIncoming(
	ctx context.Context,
	userID string,
) ([]models.Contact, error) {
	rows, err := r.db.Query(ctx,
		selectContacts+` WHERE contact_user_id = $1 AND status = 'pending'
		ORDER BY display_name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	return r.scan(rows)
}

// Add creates a pending contact request from ownerUserID to contactUserID.
func (r *ContactsRepository) Add(
	ctx context.Context,
	ownerUserID, contactUserID, displayName string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.contacts
			(owner_user_id, contact_user_id, display_name, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (owner_user_id, contact_user_id)
		DO UPDATE SET display_name = EXCLUDED.display_name`,
		ownerUserID, contactUserID, displayName,
	)
	return err
}

// Accept marks the pending request as accepted and inserts the reverse entry.
func (r *ContactsRepository) Accept(
	ctx context.Context,
	rowID uuid.UUID,
	acceptorID, displayName string,
) error {
	var ownerID string
	err := r.db.QueryRow(ctx, `
		UPDATE global.contacts SET status = 'accepted'
		WHERE id = $1 AND contact_user_id = $2
		RETURNING owner_user_id`,
		rowID, acceptorID,
	).Scan(&ownerID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO global.contacts
			(owner_user_id, contact_user_id, display_name, status)
		VALUES ($1, $2, $3, 'accepted')
		ON CONFLICT (owner_user_id, contact_user_id)
		DO UPDATE SET display_name = EXCLUDED.display_name, status = 'accepted'`,
		acceptorID, ownerID, displayName,
	)
	return err
}

// Decline deletes a pending request addressed to acceptorID.
func (r *ContactsRepository) Decline(
	ctx context.Context,
	rowID uuid.UUID,
	acceptorID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.contacts WHERE id = $1 AND contact_user_id = $2`,
		rowID, acceptorID,
	)
	return err
}

func (r *ContactsRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	ownerUserID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.contacts WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID,
	)
	return err
}

// ensure postgres import is used.
var _ = postgres.PgxErrorToHTTPError
