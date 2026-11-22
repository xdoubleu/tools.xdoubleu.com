package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"tools.xdoubleu.com/internal/kit/errortools"
)

const (
	passkeyIPKeyPrefix   = "passkey-ip:"
	passkeyUserKeyPrefix = "passkey:"
)

// errPasskeyUserLocked tells ValidatePasskeyLogin's handler apart from a bad
// assertion when the resolved account is locked out.
var errPasskeyUserLocked = errors.New("passkey account locked")

func passkeyRejected() error {
	return errortools.NewUnauthorizedError(errors.New("passkey sign-in failed"))
}

// BeginPasskeySignIn stores a challenge and returns the JSON options for
// navigator.credentials.get() with a discoverable credential: the user is
// unknown until the authenticator answers.
func (service *LocalService) BeginPasskeySignIn(ctx context.Context) ([]byte, error) {
	if service.relyingParty == nil {
		return nil, ErrPasskeysUnavailable
	}
	assertion, session, err := service.relyingParty.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	if err = service.usersStore.SavePasskeyLoginChallenge(
		ctx, session.Challenge, data, time.Now().Add(passkeyChallengeTTL),
	); err != nil {
		return nil, err
	}
	return json.Marshal(assertion.Response)
}

// FinishPasskeySignIn verifies the authenticator's assertion and mints an aal2
// session: a user-verified passkey is both factors. ip keys the lockout until
// the credential resolves to an account, whose own key then counts too. A sign
// count that is non-zero and did not increase signals a cloned authenticator
// and is rejected; zero is always accepted, as synced passkeys report it.
func (service *LocalService) FinishPasskeySignIn(
	ctx context.Context,
	ip string,
	response []byte,
) (*string, *string, error) {
	if service.relyingParty == nil {
		return nil, nil, ErrPasskeysUnavailable
	}
	ipKey := passkeyIPKeyPrefix + ip
	if err := service.attempts.check(ctx, ipKey); err != nil {
		return nil, nil, err
	}

	var (
		userKey string
		lockErr error
	)
	reject := func() (*string, *string, error) {
		if lockErr != nil {
			return nil, nil, ErrTooManyAttempts
		}
		service.attempts.fail(ctx, ipKey)
		if userKey != "" {
			service.attempts.fail(ctx, userKey)
		}
		return nil, nil, passkeyRejected()
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return reject()
	}
	data, err := service.usersStore.TakePasskeyLoginChallenge(
		ctx, parsed.Response.CollectedClientData.Challenge,
	)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return reject()
		}
		return nil, nil, err
	}
	var session webauthn.SessionData
	if err = json.Unmarshal(data, &session); err != nil {
		return nil, nil, err
	}

	user, cred, err := service.validatePasskeyLogin(
		ctx, session, parsed, func(u *passkeyUser) error {
			userKey = passkeyUserKeyPrefix + u.id
			if lockErr = service.attempts.check(ctx, userKey); lockErr != nil {
				return errPasskeyUserLocked
			}
			return nil
		},
	)
	if err != nil {
		return reject()
	}
	if cred.Authenticator.CloneWarning {
		warnClonedPasskey(ctx, user.id)
		return reject()
	}

	if err = service.usersStore.UpdatePasskeyUsage(
		ctx, user.id, cred.ID, cred.Authenticator.SignCount,
		byte(cred.Flags.ProtocolValue()),
	); err != nil {
		return nil, nil, err
	}
	service.attempts.succeed(ctx, userKey)

	accessToken, err := service.mintAccessToken(user.id, aal2)
	if err != nil {
		return nil, nil, err
	}
	refreshToken, err := service.issueRefreshToken(ctx, user.id, aal2)
	if err != nil {
		return nil, nil, err
	}
	return &accessToken, &refreshToken, nil
}

// validatePasskeyLogin runs the WebAuthn checks; onUser sees the account the
// credential resolved to before the signature is verified.
func (service *LocalService) validatePasskeyLogin(
	ctx context.Context,
	session webauthn.SessionData,
	parsed *protocol.ParsedCredentialAssertionData,
	onUser func(*passkeyUser) error,
) (*passkeyUser, *webauthn.Credential, error) {
	var resolved *passkeyUser
	_, cred, err := service.relyingParty.ValidatePasskeyLogin(
		func(_, userHandle []byte) (webauthn.User, error) {
			user, getErr := service.usersStore.GetUserByID(ctx, string(userHandle))
			if getErr != nil {
				return nil, getErr
			}
			passkeys, listErr := service.usersStore.ListPasskeys(ctx, user.ID)
			if listErr != nil {
				return nil, listErr
			}
			creds := make([]webauthn.Credential, 0, len(passkeys))
			for _, p := range passkeys {
				creds = append(creds, p.credential)
			}
			resolved = &passkeyUser{id: user.ID, email: user.Email, credentials: creds}
			if cbErr := onUser(resolved); cbErr != nil {
				return nil, cbErr
			}
			if user.Deactivated {
				return nil, errors.New("account is deactivated")
			}
			return resolved, nil
		},
		session, parsed,
	)
	return resolved, cred, err
}

// warnClonedPasskey reports a sign-in whose sign count did not increase.
func warnClonedPasskey(ctx context.Context, userID string) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelWarning)
		//nolint:exhaustruct //only the ID identifies the account
		scope.SetUser(sentry.User{ID: userID})
		hub.CaptureMessage("passkey sign count did not increase: cloned authenticator?")
	})
}

func (r *Repository) SavePasskeyLoginChallenge(
	ctx context.Context,
	challenge string,
	session []byte,
	expiresAt time.Time,
) error {
	if _, err := r.db.Exec(
		ctx, `DELETE FROM auth.passkey_login_challenges WHERE expires_at < now()`,
	); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth.passkey_login_challenges (challenge, session_data, expires_at)
		VALUES ($1, $2, $3)
	`, challenge, session, expiresAt)
	return err
}

func (r *Repository) TakePasskeyLoginChallenge(
	ctx context.Context,
	challenge string,
) ([]byte, error) {
	var data []byte
	err := r.db.QueryRow(ctx, `
		DELETE FROM auth.passkey_login_challenges
		WHERE challenge = $1 AND expires_at > now()
		RETURNING session_data
	`, challenge).Scan(&data)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return data, nil
}

func (r *Repository) UpdatePasskeyUsage(
	ctx context.Context,
	userID string,
	credentialID []byte,
	signCount uint32,
	flags byte,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth.passkeys
		SET sign_count = $3, flags = $4, last_used_at = now()
		WHERE user_id = $1 AND credential_id = $2
	`, userID, credentialID, int64(signCount), int16(flags))
	return err
}
