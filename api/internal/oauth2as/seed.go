package oauth2as

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/database/postgres"
)

// GrafanaClientID is the static Grafana SSO client seeded by migration 00045.
const GrafanaClientID = "grafana"

// grafanaSecretBCryptCost matches fosite's DefaultBCryptWorkFactor.
const grafanaSecretBCryptCost = 12

// EnsureGrafanaClientSecret writes the Grafana client's bcrypt secret_hash on
// startup (the migration leaves it NULL to keep the secret out of git).
// Idempotent; an empty secret is a no-op and a missing row is only logged.
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
