package agentaccounts

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxNameLen = 80

// Errors the service reports for a refused call; anything else is the store's.
var (
	ErrUnauthenticated = errors.New("user not authenticated")
	ErrNotAdmin        = errors.New("admin access required")
	ErrNotBrowser      = errors.New(
		"agent accounts can only be managed in a browser session",
	)
	ErrInvalidName = errors.New("name must be 1 to 80 printable characters")
	ErrInvalidID   = errors.New("invalid id")
	ErrNotSignedIn = errors.New("not signed in")
	// ErrTooManyAttempts and ErrWrongCredential are what a StepUp reports for a
	// locked-out or wrong code.
	ErrTooManyAttempts = errors.New("too many attempts")
	ErrWrongCredential = errors.New("wrong credential")
)

// Caller is who a request comes from, as the transport saw it. The zero UserID
// is an unauthenticated caller.
type Caller struct {
	UserID string
	Admin  bool
	// Browser is true for a browser session, false for a bearer token.
	Browser bool
	// AccessToken is the browser session's token, for step-up.
	AccessToken string
}

// Accounts persists agent accounts.
type Accounts interface {
	List(ctx context.Context) ([]Account, error)
	Create(ctx context.Context, createdBy, name string) (*Account, string, error)
	Revoke(ctx context.Context, id uuid.UUID) (*Account, error)
	RotateSecret(ctx context.Context, id uuid.UUID) (*Account, string, error)
	JoinFamily(ctx context.Context, id, familyID uuid.UUID) (*Account, error)
	LeaveFamily(ctx context.Context, id uuid.UUID) (*Account, error)
}

// StepUp checks a TOTP or recovery code for the signed-in user's session. It
// reports a locked-out or wrong code as ErrTooManyAttempts or ErrWrongCredential.
type StepUp interface {
	VerifyCurrentFactor(ctx context.Context, accessToken, code string) error
}

// Families resolves a person's family, created as a family-of-one if needed.
type Families interface {
	EnsureFamily(ctx context.Context, userID string) (uuid.UUID, error)
}

// Notifier tells the owner about account changes. Its methods can't fail the
// call that triggered them.
type Notifier interface {
	Created(ctx context.Context, name string)
	JoinedFamily(ctx context.Context, name string)
}

// Service is the admin-facing agent account use cases.
type Service struct {
	accounts Accounts
	stepUp   StepUp
	families Families
	notify   Notifier
}

// NewService builds a Service; notify may be nil.
func NewService(
	accounts Accounts, stepUp StepUp, families Families, notify Notifier,
) *Service {
	return &Service{
		accounts: accounts, stepUp: stepUp, families: families, notify: notify,
	}
}

// admin refuses anyone but an admin in a browser session: accounts are created
// and ended only that way, never through a bearer token.
func admin(c Caller) error {
	switch {
	case c.UserID == "":
		return ErrUnauthenticated
	case !c.Admin:
		return ErrNotAdmin
	case !c.Browser:
		return ErrNotBrowser
	}
	return nil
}

// validName trims name and rejects an empty, overlong or control-character one.
func validName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

// List returns every agent account.
func (s *Service) List(ctx context.Context, c Caller) ([]Account, error) {
	if err := admin(c); err != nil {
		return nil, err
	}
	return s.accounts.List(ctx)
}

// Create makes an account and returns its one-time client secret.
func (s *Service) Create(
	ctx context.Context, c Caller, name string,
) (*Account, string, error) {
	if err := admin(c); err != nil {
		return nil, "", err
	}
	name, ok := validName(name)
	if !ok {
		return nil, "", ErrInvalidName
	}
	account, secret, err := s.accounts.Create(ctx, "human:"+c.UserID, name)
	if err != nil {
		return nil, "", err
	}
	if s.notify != nil {
		s.notify.Created(ctx, account.Name)
	}
	return account, secret, nil
}

// Revoke ends an account.
func (s *Service) Revoke(ctx context.Context, c Caller, id string) (*Account, error) {
	if err := admin(c); err != nil {
		return nil, err
	}
	accountID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidID
	}
	return s.accounts.Revoke(ctx, accountID)
}

// RotateSecret replaces an account's client secret after a step-up check.
func (s *Service) RotateSecret(
	ctx context.Context, c Caller, id, code string,
) (*Account, string, error) {
	accountID, err := s.stepUpFor(ctx, c, id, code)
	if err != nil {
		return nil, "", err
	}
	return s.accounts.RotateSecret(ctx, accountID)
}

// AddToFamily puts an account in the admin's family after a step-up check.
func (s *Service) AddToFamily(
	ctx context.Context, c Caller, id, code string,
) (*Account, error) {
	accountID, err := s.stepUpFor(ctx, c, id, code)
	if err != nil {
		return nil, err
	}
	familyID, err := s.families.EnsureFamily(ctx, c.UserID)
	if err != nil {
		return nil, err
	}
	account, err := s.accounts.JoinFamily(ctx, accountID, familyID)
	if err != nil {
		return nil, err
	}
	if s.notify != nil {
		s.notify.JoinedFamily(ctx, account.Name)
	}
	return account, nil
}

// RemoveFromFamily takes an account out of its family.
func (s *Service) RemoveFromFamily(
	ctx context.Context, c Caller, id string,
) (*Account, error) {
	if err := admin(c); err != nil {
		return nil, err
	}
	accountID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidID
	}
	return s.accounts.LeaveFamily(ctx, accountID)
}

// stepUpFor checks the caller and id, then the code; it returns the account id.
func (s *Service) stepUpFor(
	ctx context.Context, c Caller, id, code string,
) (uuid.UUID, error) {
	if err := admin(c); err != nil {
		return uuid.Nil, err
	}
	accountID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, ErrInvalidID
	}
	if c.AccessToken == "" {
		return uuid.Nil, ErrNotSignedIn
	}
	if err = s.stepUp.VerifyCurrentFactor(ctx, c.AccessToken, code); err != nil {
		return uuid.Nil, err
	}
	return accountID, nil
}
