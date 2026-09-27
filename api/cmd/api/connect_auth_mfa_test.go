package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
)

func mfaClient(t *testing.T) authv1connect.AuthServiceClient {
	t.Helper()
	ts := connectServer(t)
	return authv1connect.NewAuthServiceClient(ts.Client(), ts.URL)
}

// enrollFactor calls MFAEnroll and returns the new factor's ID and secret.
func enrollFactor(
	t *testing.T, client authv1connect.AuthServiceClient, token http.Cookie,
) (string, string) {
	t.Helper()
	req := connect.NewRequest(&authv1.MFAEnrollRequest{})
	setCookieOnRequest(req, token)
	resp, err := client.MFAEnroll(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.FactorId)
	return resp.Msg.FactorId, resp.Msg.Secret
}

func TestMFAEnroll_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnroll(
		context.Background(),
		connect.NewRequest(&authv1.MFAEnrollRequest{}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnroll_WithToken(t *testing.T) {
	client := mfaClient(t)
	factorID, _ := enrollFactor(t, client, mfaTokenCookie)
	assert.NotEmpty(t, factorID)
}

func TestMFAEnrollVerify_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnrollVerify(context.Background(), connect.NewRequest(
		&authv1.MFAEnrollVerifyRequest{
			FactorId: uuid.NewString(),
			Code:     "123456",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollVerify_InvalidFactorID(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: "not-a-uuid",
		Code:     "123456",
	})
	setCookieOnRequest(req, mfaTokenCookie)
	_, err := client.MFAEnrollVerify(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// enrollAndVerify enrolls and verifies a factor, unenrolling on cleanup
// (one verified factor per user is allowed).
func enrollAndVerify(
	t *testing.T, client authv1connect.AuthServiceClient, token http.Cookie,
) *connect.Response[authv1.MFAEnrollVerifyResponse] {
	t.Helper()
	factorID, secret := enrollFactor(t, client, token)
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(
			context.Background(),
			`DELETE FROM auth.totp_factors WHERE id = $1`, factorID,
		)
	})

	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	})
	setCookieOnRequest(req, token)
	resp, err := client.MFAEnrollVerify(context.Background(), req)
	require.NoError(t, err)
	return resp
}

// sessionCookie returns the accessToken cookie a response set.
func sessionCookie(t *testing.T, header http.Header) http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: header}).Cookies() {
		if c.Name == "accessToken" && c.Value != "" {
			return http.Cookie{Name: c.Name, Value: c.Value}
		}
	}
	require.FailNow(t, "no accessToken Set-Cookie header")
	return http.Cookie{}
}

func TestMFAEnrollVerify_Success(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	resp := enrollAndVerify(t, client, http.Cookie{Name: "mfaToken", Value: token})
	assert.NotEmpty(t, resp.Msg.RecoveryCodes)
}

func TestMFAEnrollVerify_WithRememberMe(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	mfaToken := http.Cookie{Name: "mfaToken", Value: token}
	factorID, secret := enrollFactor(t, client, mfaToken)
	t.Cleanup(func() {
		_ = testApp.auth.UnenrollTOTP(
			context.Background(), token, uuid.MustParse(factorID),
		)
	})

	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	})
	setCookieOnRequest(
		req, mfaToken, http.Cookie{Name: "mfaRememberMe", Value: "1"},
	)
	_, err := client.MFAEnrollVerify(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAChallenge_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAChallenge(context.Background(), connect.NewRequest(
		&authv1.MFAChallengeRequest{Code: "123456"},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAChallenge_NoFactorID(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAChallengeRequest{Code: "123456"})
	setCookieOnRequest(req, mfaTokenCookie)
	_, err := client.MFAChallenge(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAChallenge_InvalidFactorID(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAChallengeRequest{Code: "123456"})
	setCookieOnRequest(req,
		mfaTokenCookie,
		http.Cookie{Name: "mfaFactorID", Value: "not-a-uuid"},
	)
	_, err := client.MFAChallenge(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestMFAChallenge_Success(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	mfaToken := http.Cookie{Name: "mfaToken", Value: token}
	factorID, secret := enrollFactor(t, client, mfaToken)
	t.Cleanup(func() {
		_ = testApp.auth.UnenrollTOTP(
			context.Background(), token, uuid.MustParse(factorID),
		)
	})

	req := connect.NewRequest(&authv1.MFAChallengeRequest{
		Code: currentTOTPCode(t, secret),
	})
	setCookieOnRequest(req,
		mfaToken,
		http.Cookie{Name: "mfaFactorID", Value: factorID},
	)
	_, err := client.MFAChallenge(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAChallenge_WithRememberMeAndRedirect(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	mfaToken := http.Cookie{Name: "mfaToken", Value: token}
	factorID, secret := enrollFactor(t, client, mfaToken)
	t.Cleanup(func() {
		_ = testApp.auth.UnenrollTOTP(
			context.Background(), token, uuid.MustParse(factorID),
		)
	})

	req := connect.NewRequest(&authv1.MFAChallengeRequest{
		Code: currentTOTPCode(t, secret),
	})
	setCookieOnRequest(req,
		mfaToken,
		http.Cookie{Name: "mfaFactorID", Value: factorID},
		http.Cookie{Name: "mfaRememberMe", Value: "1"},
		http.Cookie{Name: "mfaRedirect", Value: "/backlog"},
	)
	_, err := client.MFAChallenge(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAEnrollSkip_NoMFAToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnrollSkip(
		context.Background(),
		connect.NewRequest(&authv1.MFAEnrollSkipRequest{}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollSkip_NoRefreshToken(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollSkipRequest{})
	setCookieOnRequest(req, mfaTokenCookie)
	_, err := client.MFAEnrollSkip(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollSkip_Success(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollSkipRequest{})
	// MFAEnrollSkip only relays these cookies, so any value works.
	setCookieOnRequest(req, mfaTokenCookie, mfaRefreshTokenCookie)
	_, err := client.MFAEnrollSkip(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAEnrollSkip_WithRememberMe(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollSkipRequest{})
	setCookieOnRequest(
		req,
		mfaTokenCookie,
		mfaRefreshTokenCookie,
		http.Cookie{Name: "mfaRememberMe", Value: "1"},
	)
	_, err := client.MFAEnrollSkip(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAEnrollVerify_SettingsFlow_WithAccessToken(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	// Settings flow: accessToken cookie present, no mfaToken.
	enrollAndVerify(t, client, http.Cookie{Name: "accessToken", Value: token})
}

func TestRegenerateRecoveryCodes_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.RegenerateRecoveryCodes(
		context.Background(),
		connect.NewRequest(&authv1.RegenerateRecoveryCodesRequest{}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestRegenerateRecoveryCodes_Success(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	enrolled := enrollAndVerify(
		t, client, http.Cookie{Name: "accessToken", Value: token},
	)
	accessCookie := sessionCookie(t, enrolled.Header())

	req := connect.NewRequest(&authv1.RegenerateRecoveryCodesRequest{})
	setCookieOnRequest(req, accessCookie)
	resp, err := client.RegenerateRecoveryCodes(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.RecoveryCodes)
}

// TestRegenerateRecoveryCodes_ReplacesEarlierCodes: a second call invalidates
// the first batch.
func TestRegenerateRecoveryCodes_ReplacesEarlierCodes(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	first := enrollAndVerify(
		t, client, http.Cookie{Name: "accessToken", Value: token},
	)
	require.NotEmpty(t, first.Msg.RecoveryCodes)
	accessCookie := sessionCookie(t, first.Header())

	req := connect.NewRequest(&authv1.RegenerateRecoveryCodesRequest{})
	setCookieOnRequest(req, accessCookie)
	resp, err := client.RegenerateRecoveryCodes(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.RecoveryCodes)
	assert.NotEqual(t, first.Msg.RecoveryCodes, resp.Msg.RecoveryCodes)
}

func TestMFAEnrollVerify_SettingsFlow_PreservesRememberMe(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	accessCookie := http.Cookie{Name: "accessToken", Value: token}
	factorID, secret := enrollFactor(t, client, accessCookie)
	t.Cleanup(func() {
		_ = testApp.auth.UnenrollTOTP(
			context.Background(), token, uuid.MustParse(factorID),
		)
	})

	// Settings flow with remember-me; only the cookie's presence is checked.
	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	})
	setCookieOnRequest(
		req, accessCookie, http.Cookie{Name: "refreshToken", Value: "refresh"},
	)
	_, err := client.MFAEnrollVerify(context.Background(), req)
	require.NoError(t, err)
}

// A pre-MFA (aal1) token can't manage the factors of a user who has one.
func TestRegenerateRecoveryCodes_PreMFAToken_Unauthenticated(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.RegenerateRecoveryCodesRequest{})
	setCookieOnRequest(req, mfaAccessToken)
	_, err := client.RegenerateRecoveryCodes(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMFAEnroll_PreMFAToken_VerifiedFactor_Unauthenticated(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollRequest{})
	setCookieOnRequest(
		req, http.Cookie{Name: mfaTokenCookieName, Value: mfaAccessToken.Value},
	)
	_, err := client.MFAEnroll(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMFAEnrollSkip_VerifiedFactor_PermissionDenied(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollSkipRequest{})
	setCookieOnRequest(
		req,
		http.Cookie{Name: mfaTokenCookieName, Value: mfaAccessToken.Value},
		http.Cookie{Name: mfaRefreshTokenCookieName, Value: "unused"},
	)
	_, err := client.MFAEnrollSkip(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
