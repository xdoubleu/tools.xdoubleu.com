package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func totpCode(t *testing.T, secret string) string {
	t.Helper()
	//nolint:exhaustruct //Encoder uses the library default
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	require.NoError(t, err)
	return code
}

func TestEnrollTOTP_ProducesValidSecret(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	enrollment, err := service.EnrollTOTP(context.Background(), *access)
	require.NoError(t, err)
	assert.NotEmpty(t, enrollment.Secret)
	assert.NotEmpty(t, enrollment.QRSVG)

	// The returned secret must be usable to generate a real, valid code.
	code := totpCode(t, enrollment.Secret)
	assert.Len(t, code, 6)
}

func TestVerifyMFA_AcceptsValidCode_RejectsWrongCode(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	enrollment, err := service.EnrollTOTP(context.Background(), *access)
	require.NoError(t, err)

	// Wrong code: rejected.
	_, _, err = service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID, "000000",
	)
	require.Error(t, err)

	// Right code: accepted, and completes enrollment.
	newAccess, newRefresh, err := service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, *newAccess)
	assert.NotEmpty(t, *newRefresh)

	factorID, hasMFA := service.HasVerifiedTOTP(context.Background(), *newAccess)
	assert.True(t, hasMFA)
	assert.Equal(t, enrollment.ID, factorID)
}

func TestRecoveryCode_FallbackConsumesExactlyOnce(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	enrollment, err := service.EnrollTOTP(context.Background(), *access)
	require.NoError(t, err)
	_, _, err = service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.NoError(t, err)

	codes, err := service.GenerateRecoveryCodes(context.Background(), *access)
	require.NoError(t, err)
	require.NotEmpty(t, codes)

	// A recovery code works in place of a TOTP code.
	_, _, err = service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID, codes[0],
	)
	require.NoError(t, err)

	// The same recovery code cannot be reused.
	_, _, err = service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID, codes[0],
	)
	require.Error(t, err)
}

func TestUnenrollTOTP_ClearsFactorAndRecoveryCodes(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	enrollment, err := service.EnrollTOTP(context.Background(), *access)
	require.NoError(t, err)
	_, _, err = service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.NoError(t, err)

	_, err = service.GenerateRecoveryCodes(context.Background(), *access)
	require.NoError(t, err)

	require.NoError(t, service.UnenrollTOTP(
		context.Background(), *access, enrollment.ID,
	))

	_, hasMFA := service.HasVerifiedTOTP(context.Background(), *access)
	assert.False(t, hasMFA)

	var count int
	require.NoError(t, db.QueryRow(context.Background(), `
		SELECT count(*) FROM auth.recovery_codes WHERE user_id = $1
	`, userID).Scan(&count))
	assert.Zero(t, count)
}

func TestChallengeMFA_ReturnsFreshUUIDEachTime(t *testing.T) {
	service, _ := newTestService(t)
	c1, err := service.ChallengeMFA(context.Background(), "any", uuid.UUID{})
	require.NoError(t, err)
	c2, err := service.ChallengeMFA(context.Background(), "any", uuid.UUID{})
	require.NoError(t, err)
	assert.NotEqual(t, c1.ID, c2.ID)
}
