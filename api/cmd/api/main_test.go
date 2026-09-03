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

// testUserPassword is the fixed password every seeded test user shares.
const testUserPassword = "password"

//nolint:gochecknoglobals //needed for tests
var accessToken = http.Cookie{
	Name: "accessToken",
	// Overwritten in TestMain with a real signed JWT for testUserID — the
	// literal here only documents the cookie name until TestMain runs.
	Value: "",
}

// mfaAccessToken is a real, valid aal1 session JWT for mfaUserID — a
// dedicated user seeded with an already-verified TOTP factor, for tests that
// need to observe "this session already has MFA enabled" without mutating
// shared state (see mfaUserID's doc comment).
//
//nolint:gochecknoglobals //needed for tests
var mfaAccessToken = http.Cookie{Name: "accessToken", Value: ""}

// mfaUserID is a dedicated seeded user (distinct from testUserID) carrying
// an already-verified TOTP factor, read-only for the whole test run — tests
// that need to *mutate* MFA state (enroll, verify, unenroll) must seed their
// own throwaway user via freshTestUser instead, since this fixture is shared
// across the package and mutating it would make test order significant.
const mfaUserID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf77"

// mfaTOTPSecret is mfaUserID's known TOTP secret (base32), for tests that
// need to compute a currently-valid code for it.
const mfaTOTPSecret = "JBSWY3DPEHPK3PXP"

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	// A fixed test key so OAuth connection tests can round-trip through the
	// real AES-GCM sealer instead of the "encryption not configured" path.
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

// seedTestUsers inserts the two fixed auth.users fixtures every cmd/api test
// relies on (testUserID, mfaUserID) and mints their real session JWTs into
// the accessToken/mfaAccessToken package vars.
func seedTestUsers(ctx context.Context) error {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(testUserPassword), bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	// testUserID's email intentionally matches the historical GoTrue-mock
	// fixture ("user@example.com") — several tests assert against it via
	// global.app_users, which enrichUser resyncs from this row's email on
	// every authenticated request.
	if _, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email, password_hash = EXCLUDED.password_hash
	`, testUserID, "user@example.com", string(hash)); err != nil {
		return err
	}
	// A separate row for the SignIn-RPC tests, which authenticate with this
	// exact email/password rather than a cookie — kept distinct from
	// testUserID so mutating one (e.g. UpdatePassword tests) never affects
	// the other.
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

// freshTestUser seeds a brand-new auth.users row with a random ID/email
// (sharing testUserPassword) and returns its ID and a real signed access
// token, for tests that mutate MFA state and can't share the package-level
// fixtures above without making test order significant.
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

// freshTestUserWithRefresh is freshTestUser plus its matching real refresh
// token, for tests exercising the refresh-token cookie fallback path (which
// does a real DB lookup, unlike most other cookie values in this suite).
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

// currentTOTPCode computes a currently-valid TOTP code for the given base32
// secret, for tests that need to pass a genuinely valid code.
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
