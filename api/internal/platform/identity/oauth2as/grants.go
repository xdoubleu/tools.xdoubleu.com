package oauth2as

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
)

// ConsentGrants remembers approvals for clients that opt in with
// remember_consent, so a repeat sign-in with unchanged scopes skips consent.
type ConsentGrants interface {
	// Covers reports whether the client remembers consent and the user's
	// stored grant includes every scope.
	Covers(ctx context.Context, userID, clientID string, scopes []string) (bool, error)
	// Remember stores the approved scopes; a no-op for clients that don't opt in.
	Remember(ctx context.Context, userID, clientID string, scopes []string) error
}

// Covers implements ConsentGrants.
func (s *Store) Covers(
	ctx context.Context, userID, clientID string, scopes []string,
) (bool, error) {
	var granted []string
	err := s.db.QueryRow(ctx, `
		SELECT g.scopes
		FROM auth.oauth2_consent_grants g
		JOIN auth.oauth2_clients c ON c.id = g.client_id
		WHERE g.user_id = $1 AND g.client_id = $2 AND c.remember_consent
	`, userID, clientID).Scan(&granted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, scope := range scopes {
		if !slices.Contains(granted, scope) {
			return false, nil
		}
	}
	return true, nil
}

// Remember implements ConsentGrants. Approving replaces the stored scopes, so
// a narrower later approval drops what is no longer requested.
func (s *Store) Remember(
	ctx context.Context, userID, clientID string, scopes []string,
) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO auth.oauth2_consent_grants (user_id, client_id, scopes)
		SELECT $1, id, $3 FROM auth.oauth2_clients
		WHERE id = $2 AND remember_consent
		ON CONFLICT (user_id, client_id)
		DO UPDATE SET scopes = EXCLUDED.scopes, granted_at = now()
	`, userID, clientID, scopes)
	return err
}
