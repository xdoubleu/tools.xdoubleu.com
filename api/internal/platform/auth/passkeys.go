package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/kit/errortools"
)

const (
	passkeyChallengeTTL = 5 * time.Minute
	// passkeyRecentSignIn is how recent a sign-in must be to add or remove a
	// passkey.
	passkeyRecentSignIn = 5 * time.Minute
	maxPasskeyNameLen   = 64
	maxPasskeys         = 20
	rpDisplayName       = "tools.xdoubleu.com"
)

// ErrRecentSignInRequired rejects adding or removing a passkey when the last
// sign-in is older than passkeyRecentSignIn.
var ErrRecentSignInRequired = errors.New("sign in again to manage passkeys")

// ErrPasskeysUnavailable means WEB_URL gave no usable relying party.
var ErrPasskeysUnavailable = errors.New("passkeys are not available")

// ErrPasskeyNotFound means the user has no such passkey.
var ErrPasskeyNotFound = errors.New("passkey not found")

// ErrPasskeyInvalid rejects a registration the authenticator response or the
// challenge doesn't back, or a bad name.
var ErrPasskeyInvalid = errors.New("passkey registration failed")

// Passkey is a row from auth.passkeys.
type Passkey struct {
	ID         uuid.UUID
	UserID     string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time

	credential webauthn.Credential
}

// passkeyStore is the persistence of passkeys and their pending challenges.
type passkeyStore interface {
	CreatePasskey(
		ctx context.Context, userID, name string, cred webauthn.Credential,
	) (*Passkey, error)
	ListPasskeys(ctx context.Context, userID string) ([]Passkey, error)
	RenamePasskey(ctx context.Context, userID string, id uuid.UUID, name string) error
	DeletePasskey(ctx context.Context, userID string, id uuid.UUID) error

	SavePasskeyChallenge(
		ctx context.Context, userID string, session []byte, expiresAt time.Time,
	) error
	// TakePasskeyChallenge removes and returns the user's unexpired challenge.
	TakePasskeyChallenge(ctx context.Context, userID string) ([]byte, error)
}

var _ passkeyStore = (*Repository)(nil)

// newRelyingParty derives the WebAuthn relying party from WEB_URL: its host is
// the RP ID and its origin the only accepted one.
func newRelyingParty(webURL string) (*webauthn.WebAuthn, error) {
	u, err := url.Parse(webURL)
	if err != nil || u.Hostname() == "" || u.Scheme == "" {
		return nil, ErrPasskeysUnavailable
	}
	//nolint:exhaustruct //the other options default
	return webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: rpDisplayName,
		RPOrigins:     []string{u.Scheme + "://" + u.Host},
	})
}

type passkeyUser struct {
	id          string
	email       string
	credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte          { return []byte(u.id) }
func (u passkeyUser) WebAuthnName() string        { return u.email }
func (u passkeyUser) WebAuthnDisplayName() string { return u.email }

func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

// passkeySession authenticates the caller and returns their WebAuthn identity;
// recent additionally demands a sign-in within passkeyRecentSignIn.
func (service *LocalService) passkeySession(
	ctx context.Context,
	accessToken string,
	recent bool,
) (*passkeyUser, error) {
	if service.relyingParty == nil {
		return nil, ErrPasskeysUnavailable
	}
	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if recent && (c.AuthTime == nil ||
		time.Since(c.AuthTime.Time) > passkeyRecentSignIn) {
		return nil, ErrRecentSignInRequired
	}
	user, err := service.usersStore.GetUserByID(ctx, c.Subject)
	if err != nil {
		return nil, err
	}
	if user.Deactivated {
		return nil, errortools.NewUnauthorizedError(errors.New("account is deactivated"))
	}
	passkeys, err := service.usersStore.ListPasskeys(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(passkeys))
	for _, p := range passkeys {
		creds = append(creds, p.credential)
	}
	return &passkeyUser{id: user.ID, email: user.Email, credentials: creds}, nil
}

// BeginPasskeyRegistration stores a challenge for the user and returns the JSON
// options for navigator.credentials.create().
func (service *LocalService) BeginPasskeyRegistration(
	ctx context.Context,
	accessToken string,
) ([]byte, error) {
	user, err := service.passkeySession(ctx, accessToken, true)
	if err != nil {
		return nil, err
	}
	if len(user.credentials) >= maxPasskeys {
		return nil, fmt.Errorf("%w: at most %d passkeys", ErrPasskeyInvalid, maxPasskeys)
	}

	creation, session, err := service.relyingParty.BeginRegistration(
		user,
		webauthn.WithExclusions(
			webauthn.Credentials(user.credentials).CredentialDescriptors(),
		),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	if err = service.usersStore.SavePasskeyChallenge(
		ctx, user.id, data, time.Now().Add(passkeyChallengeTTL),
	); err != nil {
		return nil, err
	}
	return json.Marshal(creation.Response)
}

// FinishPasskeyRegistration verifies the authenticator's response against the
// user's pending challenge, the relying party and the origin, then stores the
// credential under name. The challenge is single-use.
func (service *LocalService) FinishPasskeyRegistration(
	ctx context.Context,
	accessToken, name string,
	response []byte,
) (*Passkey, error) {
	name, err := cleanPasskeyName(name)
	if err != nil {
		return nil, err
	}
	user, err := service.passkeySession(ctx, accessToken, true)
	if err != nil {
		return nil, err
	}
	data, err := service.usersStore.TakePasskeyChallenge(ctx, user.id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, fmt.Errorf("%w: no pending challenge", ErrPasskeyInvalid)
		}
		return nil, err
	}
	var session webauthn.SessionData
	if err = json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPasskeyInvalid, err)
	}
	cred, err := service.relyingParty.CreateCredential(user, session, parsed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPasskeyInvalid, err)
	}
	return service.usersStore.CreatePasskey(ctx, user.id, name, *cred)
}

// ListPasskeys returns the user's passkeys, oldest first.
func (service *LocalService) ListPasskeys(
	ctx context.Context,
	accessToken string,
) ([]Passkey, error) {
	user, err := service.passkeySession(ctx, accessToken, false)
	if err != nil {
		return nil, err
	}
	return service.usersStore.ListPasskeys(ctx, user.id)
}

// RenamePasskey renames one of the user's passkeys.
func (service *LocalService) RenamePasskey(
	ctx context.Context,
	accessToken string,
	id uuid.UUID,
	name string,
) error {
	name, err := cleanPasskeyName(name)
	if err != nil {
		return err
	}
	user, err := service.passkeySession(ctx, accessToken, false)
	if err != nil {
		return err
	}
	return service.usersStore.RenamePasskey(ctx, user.id, id, name)
}

// RemovePasskey deletes one of the user's passkeys.
func (service *LocalService) RemovePasskey(
	ctx context.Context,
	accessToken string,
	id uuid.UUID,
) error {
	user, err := service.passkeySession(ctx, accessToken, true)
	if err != nil {
		return err
	}
	return service.usersStore.DeletePasskey(ctx, user.id, id)
}

func cleanPasskeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxPasskeyNameLen {
		return "", fmt.Errorf(
			"%w: name must be 1-%d characters", ErrPasskeyInvalid, maxPasskeyNameLen,
		)
	}
	return name, nil
}

//nolint:gosec //column names, not credentials
const passkeyColumns = `id, user_id, name, created_at, last_used_at, credential_id,
	public_key, attestation_type, aaguid, flags, sign_count, transports`

func scanPasskey(row pgx.Row) (*Passkey, error) {
	var (
		p          Passkey
		flags      int16
		signCount  int64
		transports []string
	)
	cred := &p.credential
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.LastUsedAt, &cred.ID,
		&cred.PublicKey, &cred.AttestationType, &cred.Authenticator.AAGUID,
		&flags, &signCount, &transports,
	); err != nil {
		return nil, err
	}
	//nolint:gosec //stored from one byte
	cred.Flags = webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(flags))
	cred.Authenticator.SignCount = uint32(signCount) //nolint:gosec // stored from a uint32
	for _, t := range transports {
		cred.Transport = append(cred.Transport, protocol.AuthenticatorTransport(t))
	}
	return &p, nil
}

func (r *Repository) CreatePasskey(
	ctx context.Context,
	userID, name string,
	cred webauthn.Credential,
) (*Passkey, error) {
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	p, err := scanPasskey(r.db.QueryRow(ctx, `
		INSERT INTO auth.passkeys (user_id, name, credential_id, public_key,
			attestation_type, aaguid, flags, sign_count, transports)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+passkeyColumns,
		userID, name, cred.ID, cred.PublicKey, cred.AttestationType,
		cred.Authenticator.AAGUID, int16(cred.Flags.ProtocolValue()),
		int64(cred.Authenticator.SignCount), transports,
	))
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) ListPasskeys(
	ctx context.Context,
	userID string,
) ([]Passkey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+passkeyColumns+` FROM auth.passkeys
		WHERE user_id = $1 ORDER BY created_at, id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	passkeys := []Passkey{}
	for rows.Next() {
		p, scanErr := scanPasskey(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		passkeys = append(passkeys, *p)
	}
	return passkeys, rows.Err()
}

func (r *Repository) RenamePasskey(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE auth.passkeys SET name = $3 WHERE id = $1 AND user_id = $2
	`, id, userID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func (r *Repository) DeletePasskey(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM auth.passkeys WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func (r *Repository) SavePasskeyChallenge(
	ctx context.Context,
	userID string,
	session []byte,
	expiresAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.passkey_challenges (user_id, session_data, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id)
		DO UPDATE SET session_data = $2, expires_at = $3
	`, userID, session, expiresAt)
	return err
}

func (r *Repository) TakePasskeyChallenge(
	ctx context.Context,
	userID string,
) ([]byte, error) {
	var data []byte
	err := r.db.QueryRow(ctx, `
		DELETE FROM auth.passkey_challenges
		WHERE user_id = $1 AND expires_at > now()
		RETURNING session_data
	`, userID).Scan(&data)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return data, nil
}
