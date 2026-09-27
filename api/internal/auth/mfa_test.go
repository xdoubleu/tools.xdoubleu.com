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

	"tools.xdoubleu.com/internal/auth"
)

func totpCode(t *testing.T, secret string) string {
	t.Helper()
	return totpCodeAt(t, secret, 0)
}

// totpCodeAt is the code offsetSteps periods from now; each step is accepted
// once.
func totpCodeAt(t *testing.T, secret string, offsetSteps int) string {
	t.Helper()
	at := time.Now().Add(time.Duration(offsetSteps) * 30 * time.Second)
	//nolint:exhaustruct //Encoder uses the library default
	code, err := totp.GenerateCodeCustom(secret, at, totp.ValidateOpts{
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
	session, _, err := service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.NoError(t, err)

	codes, err := service.GenerateRecoveryCodes(context.Background(), *session)
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
	session, _, err := service.VerifyMFA(
		context.Background(), *access, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.NoError(t, err)

	_, err = service.GenerateRecoveryCodes(context.Background(), *session)
	require.NoError(t, err)

	require.NoError(t, service.UnenrollTOTP(
		context.Background(), *session, enrollment.ID,
	))

	_, hasMFA := service.HasVerifiedTOTP(context.Background(), *access)
	assert.False(t, hasMFA)

	var count int
	require.NoError(t, db.QueryRow(context.Background(), `
		SELECT count(*) FROM auth.recovery_codes WHERE user_id = $1
	`, userID).Scan(&count))
	assert.Zero(t, count)
}

func TestEnrollTOTP_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)
	_, err := service.EnrollTOTP(context.Background(), "not-a-real-token")
	require.Error(t, err)
}

func TestUnenrollTOTP_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)
	err := service.UnenrollTOTP(
		context.Background(), "not-a-real-token", uuid.UUID{},
	)
	require.Error(t, err)
}

func TestGenerateRecoveryCodes_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)
	_, err := service.GenerateRecoveryCodes(context.Background(), "not-a-real-token")
	require.Error(t, err)
}

func TestVerifyMFA_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)
	_, _, err := service.VerifyMFA(
		context.Background(), "not-a-real-token", uuid.New(), uuid.New(), "123456",
	)
	require.Error(t, err)
}

func TestVerifyMFA_UnknownFactorID(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	_, _, err = service.VerifyMFA(
		context.Background(), *access, uuid.New(), uuid.New(), "123456",
	)
	require.Error(t, err)
}

func TestVerifyMFA_FactorBelongsToDifferentUser(t *testing.T) {
	service, db := newTestService(t)
	ownerID := seedUser(t, db)
	ownerAccess, _, err := service.SignInWithEmail(
		context.Background(), ownerID+"@example.com", testPassword,
	)
	require.NoError(t, err)
	enrollment, err := service.EnrollTOTP(context.Background(), *ownerAccess)
	require.NoError(t, err)

	otherID := seedUser(t, db)
	otherAccess, _, err := service.SignInWithEmail(
		context.Background(), otherID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	// otherAccess belongs to a different user than the factor.
	_, _, err = service.VerifyMFA(
		context.Background(), *otherAccess, enrollment.ID, enrollment.ID,
		totpCode(t, enrollment.Secret),
	)
	require.Error(t, err)
}

func TestChallengeMFA_ReturnsFreshUUIDEachTime(t *testing.T) {
	service, _ := newTestService(t)
	c1, err := service.ChallengeMFA(context.Background(), "any", uuid.UUID{})
	require.NoError(t, err)
	c2, err := service.ChallengeMFA(context.Background(), "any", uuid.UUID{})
	require.NoError(t, err)
	assert.NotEqual(t, c1.ID, c2.ID)
}

// enrolledUser signs in a fresh user and verifies a TOTP factor, returning the
// pre-MFA (aal1) access and refresh tokens plus the enrollment.
func enrolledUser(
	t *testing.T,
) (*auth.LocalService, string, string, *auth.TOTPEnrollment) {
	t.Helper()
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, refresh, err := service.SignInWithEmail(
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
	return service, *access, *refresh, enrollment
}

func TestGetUser_PreMFAToken_Rejected(t *testing.T) {
	service, access, _, _ := enrolledUser(t)
	_, err := service.GetUser(context.Background(), access)
	require.Error(t, err)
}

func TestSignInWithRefreshToken_PreMFAToken_Rejected(t *testing.T) {
	service, _, refresh, _ := enrolledUser(t)
	_, _, err := service.SignInWithRefreshToken(context.Background(), refresh)
	require.Error(t, err)
}

func TestUpdatePassword_PreMFAToken_Rejected(t *testing.T) {
	service, access, _, _ := enrolledUser(t)
	require.Error(
		t,
		service.UpdatePassword(
			context.Background(), access, testPassword, "new-password",
		),
	)
}

func TestEnrollTOTP_PreMFAToken_Rejected(t *testing.T) {
	service, access, _, _ := enrolledUser(t)
	_, err := service.EnrollTOTP(context.Background(), access)
	require.Error(t, err)
}

// The pre-MFA token can pass the challenge but can't verify a second factor.
func TestVerifyMFA_PreMFAToken_CannotVerifyNewFactor(t *testing.T) {
	service, access, _, enrollment := enrolledUser(t)
	session, _, err := service.VerifyMFA(
		context.Background(), access, enrollment.ID, enrollment.ID,
		totpCodeAt(t, enrollment.Secret, 1),
	)
	require.NoError(t, err)

	second, err := service.EnrollTOTP(context.Background(), *session)
	require.NoError(t, err)

	_, _, err = service.VerifyMFA(
		context.Background(), access, second.ID, second.ID,
		totpCode(t, second.Secret),
	)
	require.Error(t, err)
}

func TestValidateAccessToken(t *testing.T) {
	service, access, _, _ := enrolledUser(t)
	require.NoError(t, service.ValidateAccessToken(access))
	require.Error(t, service.ValidateAccessToken("not-a-jwt"))
}

func TestVerifyMFA_RejectsReplayedCode(t *testing.T) {
	service, access, _, enrollment := enrolledUser(t)
	code := totpCodeAt(t, enrollment.Secret, 1)

	_, _, err := service.VerifyMFA(
		context.Background(), access, enrollment.ID, enrollment.ID, code,
	)
	require.NoError(t, err)
	_, _, err = service.VerifyMFA(
		context.Background(), access, enrollment.ID, enrollment.ID, code,
	)
	require.Error(t, err)

	// An earlier step than the last accepted one is a replay too.
	_, _, err = service.VerifyMFA(
		context.Background(), access, enrollment.ID, enrollment.ID,
		totpCodeAt(t, enrollment.Secret, 0),
	)
	require.Error(t, err)
}

func TestVerifyCurrentFactor(t *testing.T) {
	service, access, _, enrollment := enrolledUser(t)
	session, _, err := service.VerifyMFA(
		context.Background(), access, enrollment.ID, enrollment.ID,
		totpCodeAt(t, enrollment.Secret, 1),
	)
	require.NoError(t, err)

	assert.ErrorIs(t,
		service.VerifyCurrentFactor(context.Background(), *session, "000000"),
		auth.ErrWrongCredential,
	)
	codes, err := service.GenerateRecoveryCodes(context.Background(), *session)
	require.NoError(t, err)
	require.NoError(t,
		service.VerifyCurrentFactor(context.Background(), *session, codes[0]),
	)
	// A pre-MFA token can't pass the step-up.
	require.Error(t,
		service.VerifyCurrentFactor(context.Background(), access, codes[1]),
	)
}

func TestVerifyCurrentFactor_NoFactor(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)
	assert.ErrorIs(t,
		service.VerifyCurrentFactor(context.Background(), *access, "123456"),
		auth.ErrWrongCredential,
	)
}
