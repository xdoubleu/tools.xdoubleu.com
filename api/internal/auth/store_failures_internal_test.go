package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errStore = errors.New("store down")

// racyStore stands in for a token another request consumed first, or a
// failing write; unused usersStore methods panic.
type racyStore struct {
	usersStore
	consumed   bool
	writeErr   error
	revokeErr  error
	getUserErr error
}

func (s racyStore) GetRefreshTokenByHash(
	context.Context, string,
) (*RefreshTokenRow, error) {
	return &RefreshTokenRow{
		ID: uuid.New(), UserID: "user", AAL: aal2,
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (s racyStore) DeleteRefreshToken(context.Context, uuid.UUID) (bool, error) {
	return s.consumed, s.writeErr
}

func (s racyStore) GetPasswordResetTokenByHash(
	context.Context, string,
) (*PasswordResetTokenRow, error) {
	//nolint:exhaustruct // UsedAt nil: not yet used when read
	return &PasswordResetTokenRow{
		ID: uuid.New(), UserID: "user", ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (s racyStore) MarkPasswordResetTokenUsed(
	context.Context, uuid.UUID,
) (bool, error) {
	return s.consumed, s.writeErr
}

func (s racyStore) SetPasswordHash(context.Context, string, string) error {
	return nil
}

func (s racyStore) DeleteAllRefreshTokensForUser(context.Context, string) error {
	return nil
}

func (s racyStore) RevokeOAuthGrants(context.Context, string) error {
	return s.revokeErr
}

func (s racyStore) GetVerifiedTOTPFactor(context.Context, string) (*TOTPFactor, error) {
	return nil, errNotFound
}

func (s racyStore) GetUserByID(context.Context, string) (*User, error) {
	return nil, s.getUserErr
}

func racyService(store racyStore) *LocalService {
	//nolint:exhaustruct // only the fields these paths read
	return &LocalService{
		usersStore: store, jwtSecret: []byte("secret"),
		accessExpiry: "1h", refreshExpiry: "1h",
		userCache: newUserCache(time.Minute),
	}
}

func TestSignInWithRefreshToken_AlreadyRotated(t *testing.T) {
	service := racyService(racyStore{consumed: false}) //nolint:exhaustruct // fixture
	_, _, err := service.SignInWithRefreshToken(t.Context(), "token")
	require.Error(t, err)
}

func TestSignInWithRefreshToken_DeleteFails(t *testing.T) {
	//nolint:exhaustruct // fixture
	service := racyService(racyStore{writeErr: errStore})
	_, _, err := service.SignInWithRefreshToken(t.Context(), "token")
	assert.ErrorIs(t, err, errStore)
}

func TestResetPasswordWithToken_AlreadyConsumed(t *testing.T) {
	service := racyService(racyStore{consumed: false}) //nolint:exhaustruct // fixture
	err := service.ResetPasswordWithToken(t.Context(), "token", "long-enough")
	require.Error(t, err)
}

func TestResetPasswordWithToken_ConsumeFails(t *testing.T) {
	//nolint:exhaustruct // fixture
	service := racyService(racyStore{writeErr: errStore})
	err := service.ResetPasswordWithToken(t.Context(), "token", "long-enough")
	assert.ErrorIs(t, err, errStore)
}

func TestResetPasswordWithToken_RevokeFails(t *testing.T) {
	//nolint:exhaustruct // fixture
	service := racyService(racyStore{consumed: true, revokeErr: errStore})
	err := service.ResetPasswordWithToken(t.Context(), "token", "long-enough")
	assert.ErrorIs(t, err, errStore)
}

func TestUpdatePassword_UserLookupFails(t *testing.T) {
	//nolint:exhaustruct // fixture
	service := racyService(racyStore{getUserErr: errStore})
	token, err := service.mintAccessToken("user", aal2)
	require.NoError(t, err)
	err = service.UpdatePassword(t.Context(), token, "current", "long-enough")
	assert.ErrorIs(t, err, errStore)
}
