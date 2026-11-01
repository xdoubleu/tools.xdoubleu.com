package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/database"
)

const (
	// PairingTTL is how long a pairing code works.
	PairingTTL    = 10 * time.Minute
	maxRunnerName = 80

	// Crockford's base32 leaves out I, L, O and U, so a code read aloud or off a
	// phone has few look-alikes.
	codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	codeLength   = 10
	tokenBytes   = 32
	// TokenPrefix marks a device token, so a leaked one is easy to recognise.
	TokenPrefix = "frt_"
)

type RunnerService struct {
	repo *repositories.RunnersRepository
	now  func() time.Time
}

// newCode returns a pairing code as two groups of five characters.
func newCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range buf {
		if i == codeLength/2 {
			b.WriteByte('-')
		}
		b.WriteByte(codeAlphabet[int(v)%len(codeAlphabet)])
	}
	return b.String(), nil
}

// normaliseCode upper-cases a typed code, drops dashes and spaces, and reads
// the look-alikes O as 0 and I or L as 1, as Crockford's base32 does.
func normaliseCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		switch {
		case r == '-' || unicode.IsSpace(r):
		case r == 'O':
			b.WriteByte('0')
		case r == 'I' || r == 'L':
			b.WriteByte('1')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hashSecret is the stored form of a pairing code or device token: both are
// random, so an unsalted SHA-256 is enough and lets the database look one up.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func validRunnerName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxRunnerName {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

// Create adds a runner waiting to pair and returns its single-use code, which
// can't be shown again. Another active runner is a conflict: revoke it first.
func (s *RunnerService) Create(
	ctx context.Context,
	createdBy, name string,
) (*models.Runner, string, error) {
	name, ok := validRunnerName(name)
	if !ok {
		return nil, "", badRequest("name must be 1 to 80 printable characters")
	}
	code, err := newCode()
	if err != nil {
		return nil, "", err
	}
	runner, err := s.repo.Create(ctx, name, createdBy,
		hashSecret(normaliseCode(code)), s.now().Add(PairingTTL))
	if err != nil {
		return nil, "", err
	}
	return runner, code, nil
}

// Pair trades a pairing code for a device token, returned once.
func (s *RunnerService) Pair(
	ctx context.Context,
	code, version string,
) (string, *models.Runner, error) {
	normalised := normaliseCode(code)
	if len(normalised) != codeLength {
		return "", nil, badRequest("invalid pairing code")
	}
	token, err := newToken()
	if err != nil {
		return "", nil, err
	}
	runner, err := s.repo.Pair(ctx, hashSecret(normalised), hashSecret(token), version)
	if err != nil {
		return "", nil, err
	}
	return token, runner, nil
}

// Authenticate returns the active runner a device token belongs to; any other
// token is not found.
func (s *RunnerService) Authenticate(
	ctx context.Context,
	token string,
) (*models.Runner, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return nil, database.ErrResourceNotFound
	}
	return s.repo.ByToken(ctx, hashSecret(token))
}

// Heartbeat records that the runner is alive.
func (s *RunnerService) Heartbeat(
	ctx context.Context,
	id uuid.UUID,
	version string,
) error {
	return s.repo.Touch(ctx, id, version)
}

// List returns every runner.
func (s *RunnerService) List(ctx context.Context) ([]models.Runner, error) {
	return s.repo.List(ctx)
}

// Revoke ends a runner at once.
func (s *RunnerService) Revoke(
	ctx context.Context,
	id uuid.UUID,
) (*models.Runner, error) {
	return s.repo.Revoke(ctx, id)
}

// Now is the service's clock, for turning a runner into its online state.
func (s *RunnerService) Now() time.Time {
	return s.now()
}
