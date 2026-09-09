package oauth2as

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/database/postgres"
)

// GrafanaClientID is the fixed client_id of the static confidential client
// seeded by migration 00045 for Grafana SSO (issue #1469).
const GrafanaClientID = "grafana"

// grafanaSecretBCryptCost matches fosite's DefaultBCryptWorkFactor, so a hash
// written here verifies identically to one fosite would have produced.
const grafanaSecretBCryptCost = 12

// EnsureGrafanaClientSecret reconciles auth.oauth2_clients' secret_hash for
// the static Grafana client with the configured plaintext secret. The seed
// migration inserts the client row with a NULL secret_hash on purpose — the
// plaintext secret must not live in version control — so this runs on every
// startup to write (or, on rotation, rewrite) the bcrypt hash.
//
// It is idempotent: once the stored hash already verifies against secret,
// nothing is written. An empty secret is a no-op (Grafana SSO simply stays
// unusable until OAUTH_GRAFANA_CLIENT_SECRET is set), and a missing client
// row (migration not yet applied) is logged, not fatal.
func EnsureGrafanaClientSecret(
	ctx context.Context,
	db postgres.DB,
	secret string,
	logger *slog.Logger,
) error {
	if secret == "" {
		if logger != nil {
			logger.InfoContext(
				ctx,
				"OAUTH_GRAFANA_CLIENT_SECRET is unset — Grafana SSO client cannot authenticate",
				slog.String("client_id", GrafanaClientID),
			)
		}
		return nil
	}

	var existingHash *string
	err := db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`, GrafanaClientID,
	).Scan(&existingHash)
	if errors.Is(err, pgx.ErrNoRows) {
		if logger != nil {
			logger.WarnContext(ctx,
				"Grafana OAuth client row is missing; skipping secret reconciliation",
				slog.String("client_id", GrafanaClientID),
			)
		}
		return nil
	}
	if err != nil {
		return err
	}

	if existingHash != nil &&
		bcrypt.CompareHashAndPassword([]byte(*existingHash), []byte(secret)) == nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), grafanaSecretBCryptCost)
	if err != nil {
		return err
	}
	if _, err = db.Exec(ctx,
		`UPDATE auth.oauth2_clients SET secret_hash = $2 WHERE id = $1`,
		GrafanaClientID, string(hash),
	); err != nil {
		return err
	}

	if logger != nil {
		logger.InfoContext(ctx, "Grafana OAuth client secret reconciled",
			slog.String("client_id", GrafanaClientID),
		)
	}
	return nil
}
