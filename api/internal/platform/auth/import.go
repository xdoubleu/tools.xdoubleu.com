package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/kit/crypto"
)

const importRunEvery = time.Hour

// ImportCandidate is a person not yet linked to the identity provider, with the
// credentials the provider can take over.
type ImportCandidate struct {
	UserID       string
	Email        string
	DisplayName  string
	PasswordHash string
	// SealedTOTP is the verified factor's sealed secret, empty without 2FA.
	SealedTOTP string
}

// importStore lists the people to import and records their subject.
type importStore interface {
	ListImportCandidates(ctx context.Context) ([]ImportCandidate, error)
	SetOIDCSubject(ctx context.Context, userID, subject string) error
}

// Importer creates a person at the provider with their existing credentials.
type Importer interface {
	ImportHuman(
		ctx context.Context,
		email, displayName, passwordHash, totpSecret string,
	) (string, error)
}

// ImportJob moves every person's bcrypt hash, TOTP secret and verified email to
// the provider and stores their subject (adr-0051). Recovery codes don't carry
// over. Agent and service users, deactivated people and people already linked
// are skipped, so a rerun only finishes what a failure left.
type ImportJob struct {
	store    importStore
	importer Importer
	sealer   *crypto.Sealer
}

func NewImportJob(
	store importStore,
	importer Importer,
	sealer *crypto.Sealer,
) *ImportJob {
	return &ImportJob{store: store, importer: importer, sealer: sealer}
}

func (j *ImportJob) ID() string {
	return "import-users-to-identity-provider"
}

func (j *ImportJob) RunEvery() time.Duration {
	return importRunEvery
}

// Run imports every candidate and returns all failures together, so one bad
// hash doesn't hold the others back.
func (j *ImportJob) Run(ctx context.Context, logger *slog.Logger) error {
	candidates, err := j.store.ListImportCandidates(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range candidates {
		if err = j.importOne(ctx, c); err != nil {
			errs = append(errs, fmt.Errorf("import user %s: %w", c.UserID, err))
			continue
		}
		logger.InfoContext(ctx, "imported user to identity provider", "user", c.UserID)
	}
	return errors.Join(errs...)
}

func (j *ImportJob) importOne(ctx context.Context, c ImportCandidate) error {
	secret, err := j.totpSecret(c.SealedTOTP)
	if err != nil {
		return err
	}
	subject, err := j.importer.ImportHuman(
		ctx, c.Email, c.DisplayName, c.PasswordHash, secret,
	)
	if err != nil {
		return err
	}
	return j.store.SetOIDCSubject(ctx, c.UserID, subject)
}

func (j *ImportJob) totpSecret(sealedB64 string) (string, error) {
	if sealedB64 == "" {
		return "", nil
	}
	sealed, err := base64.StdEncoding.DecodeString(sealedB64)
	if err != nil {
		return "", err
	}
	secret, err := j.sealer.Decrypt(sealed)
	if err != nil {
		return "", err
	}
	return string(secret), nil
}
