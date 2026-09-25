package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/testhelper"
)

var testApp *Application //nolint:gochecknoglobals //needed for tests

const testUserPassword = "password"

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name: "accessToken",
	// Set to a real JWT in TestMain.
	Value: "",
}

// mfaAccessToken is a real aal1 session JWT for mfaUserID.
//
//nolint:gochecknoglobals //needed for tests
var mfaAccessToken = http.Cookie{Name: "accessToken", Value: ""}

// mfaUserID is a shared, read-only user with a verified TOTP factor. Tests
// that mutate MFA state must use freshTestUser.
const mfaUserID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf77"

// mfaTOTPSecret is mfaUserID's base32 TOTP secret.
const mfaTOTPSecret = "JBSWY3DPEHPK3PXP"

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	// A fixed key so OAuth tests use the real AES-GCM sealer.
	cfg.EncryptionKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

	postgresDB, err := newDBPool(logging.NewNopLogger(), cfg.DBDsn)
	if err != nil {
		panic(err)
	}

	testApp = NewApplication(logging.NewNopLogger(), cfg, postgresDB)

	ctx := context.Background()
	if _, err = postgresDB.Exec(
		ctx,
		"DELETE FROM global.family_invites WHERE to_user_id = $1",
		testUserID,
	); err != nil {
		panic(err)
	}
	if _, err = postgresDB.Exec(
		ctx,
		"DELETE FROM global.family_members WHERE user_id = $1",
		testUserID,
	); err != nil {
		panic(err)
	}

	if err = seedTestUsers(ctx); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// seedTestUsers inserts testUserID and mfaUserID and mints their JWTs.
func seedTestUsers(ctx context.Context) error {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(testUserPassword), bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	// Several tests assert on this email via global.app_users.
	if _, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email, password_hash = EXCLUDED.password_hash
	`, testUserID, "user@example.com", string(hash)); err != nil {
		return err
	}
	// A separate row for SignIn-RPC tests, so password changes elsewhere don't
	// affect them.
	if _, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES (gen_random_uuid(), $1, $2)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
	`, "valid@example.com", string(hash)); err != nil {
		return err
	}
	if _, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email, password_hash = EXCLUDED.password_hash
	`, mfaUserID, "mfa-user@example.com", string(hash)); err != nil {
		return err
	}

	sealer, err := crypto.New(testApp.config.EncryptionKey)
	if err != nil {
		return err
	}
	sealed, err := sealer.Encrypt([]byte(mfaTOTPSecret))
	if err != nil {
		return err
	}
	if _, err = testApp.db.Exec(
		ctx, `DELETE FROM auth.totp_factors WHERE user_id = $1`, mfaUserID,
	); err != nil {
		return err
	}
	if _, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.totp_factors (user_id, secret, status)
		VALUES ($1, $2, 'verified')
	`, mfaUserID, base64.StdEncoding.EncodeToString(sealed)); err != nil {
		return err
	}

	tok, _, err := testApp.auth.SignInWithEmail(
		ctx,
		"user@example.com",
		testUserPassword,
	)
	if err != nil {
		return err
	}
	accessToken.Value = *tok
	mfaTokenCookie.Value = *tok

	mfaTok, _, err := testApp.auth.SignInWithEmail(
		ctx, "mfa-user@example.com", testUserPassword,
	)
	if err != nil {
		return err
	}
	mfaAccessToken.Value = *mfaTok

	return nil
}

// freshTestUser seeds a random user and returns its access token, for tests
// that mutate shared state.
func freshTestUser(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	id := uuid.New().String()
	email := id + "@example.com"

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(testUserPassword), bcrypt.DefaultCost,
	)
	require.NoError(t, err)

	_, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
	`, id, email, string(hash))
	require.NoError(t, err)

	tok, _, err := testApp.auth.SignInWithEmail(ctx, email, testUserPassword)
	require.NoError(t, err)

	return *tok
}

// freshTestUserWithRefresh adds a real refresh token (that path does a DB
// lookup).
func freshTestUserWithRefresh(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	id := uuid.New().String()
	email := id + "@example.com"

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(testUserPassword), bcrypt.DefaultCost,
	)
	require.NoError(t, err)

	_, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
	`, id, email, string(hash))
	require.NoError(t, err)

	_, refresh, err := testApp.auth.SignInWithEmail(ctx, email, testUserPassword)
	require.NoError(t, err)

	return *refresh
}

const (
	totpPeriod = 30 * time.Second
	totpSkew   = 1
)

// currentTOTPCode computes a valid TOTP code for secret.
func currentTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	//nolint:exhaustruct //Encoder uses the library default
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Period:    uint(totpPeriod.Seconds()),
		Skew:      totpSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	require.NoError(t, err)
	return code
}
