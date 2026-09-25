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

// RoutinesClientID is the static machine client seeded by migration 00053. Its
// client_credentials tokens act as RoutinesServiceUserID.
const RoutinesClientID = "routines"

// RoutinesServiceUserID is the seeded service-role user behind RoutinesClientID.
const RoutinesServiceUserID = "131220c3-7027-4f72-b714-b6ecb114d19d"

// machineSubject maps a client_credentials client to the user its tokens act
// as; a token without a subject is rejected by ResolveAccessToken.
func machineSubject(clientID string) (string, bool) {
	if clientID == RoutinesClientID {
		return RoutinesServiceUserID, true
	}
	return "", false
}

// clientSecretBCryptCost matches fosite's DefaultBCryptWorkFactor.
const clientSecretBCryptCost = 12

// EnsureGrafanaClientSecret writes the Grafana client's secret_hash from
// OAUTH_GRAFANA_CLIENT_SECRET.
func EnsureGrafanaClientSecret(
	ctx context.Context,
	db postgres.DB,
	secret string,
	logger *slog.Logger,
) error {
	return ensureClientSecret(
		ctx, db, GrafanaClientID, "OAUTH_GRAFANA_CLIENT_SECRET", secret, logger,
	)
}

// EnsureRoutinesClientSecret writes the routines client's secret_hash from
// OAUTH_ROUTINES_CLIENT_SECRET.
func EnsureRoutinesClientSecret(
	ctx context.Context,
	db postgres.DB,
	secret string,
	logger *slog.Logger,
) error {
	return ensureClientSecret(
		ctx, db, RoutinesClientID, "OAUTH_ROUTINES_CLIENT_SECRET", secret, logger,
	)
}

// ensureClientSecret writes a static client's bcrypt secret_hash on startup
// (its migration leaves it NULL to keep the secret out of git). Idempotent; an
// empty secret is a no-op and a missing row is only logged.
func ensureClientSecret(
	ctx context.Context,
	db postgres.DB,
	clientID, envName, secret string,
	logger *slog.Logger,
) error {
	if secret == "" {
		if logger != nil {
			logger.InfoContext(
				ctx,
				envName+" is unset — OAuth client cannot authenticate",
				slog.String("client_id", clientID),
			)
		}
		return nil
	}

	var existingHash *string
	err := db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`, clientID,
	).Scan(&existingHash)
	if errors.Is(err, pgx.ErrNoRows) {
		if logger != nil {
			logger.WarnContext(ctx,
				"OAuth client row is missing; skipping secret reconciliation",
				slog.String("client_id", clientID),
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

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), clientSecretBCryptCost)
	if err != nil {
		return err
	}
	if _, err = db.Exec(ctx,
		`UPDATE auth.oauth2_clients SET secret_hash = $2 WHERE id = $1`,
		clientID, string(hash),
	); err != nil {
		return err
	}

	if logger != nil {
		logger.InfoContext(ctx, "OAuth client secret reconciled",
			slog.String("client_id", clientID),
		)
	}
	return nil
}
