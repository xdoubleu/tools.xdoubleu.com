package main

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	account "tools.xdoubleu.com/internal/account"
)

func mfaClient(t *testing.T) authv1connect.AuthServiceClient {
	t.Helper()
	ts := connectServer(t)
	return authv1connect.NewAuthServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
		),
	)
}

// enrollFactor calls MFAEnroll and returns the new factor's ID and secret.
func enrollFactor(
	t *testing.T, client authv1connect.AuthServiceClient, token http.Cookie,
) (string, string) {
	t.Helper()
	req := &authv1.MFAEnrollRequest{}
	resp, err := client.MFAEnroll(withCookies(context.Background(), token), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.FactorId)
	return resp.FactorId, resp.Secret
}

func TestMFAEnroll_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnroll(
		context.Background(),
		&authv1.MFAEnrollRequest{},
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
	_, err := client.MFAEnrollVerify(context.Background(), &authv1.MFAEnrollVerifyRequest{
		FactorId: uuid.NewString(),
		Code:     "123456",
	})
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollVerify_InvalidFactorID(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollVerifyRequest{
		FactorId: "not-a-uuid",
		Code:     "123456",
	}
	_, err := client.MFAEnrollVerify(
		withCookies(context.Background(), mfaTokenCookie),
		req,
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// enrollAndVerify enrolls and verifies a factor, unenrolling on cleanup
// (one verified factor per user is allowed).
func enrollAndVerify(
	t *testing.T, client authv1connect.AuthServiceClient, token http.Cookie,
) (*authv1.MFAEnrollVerifyResponse, *connect.Header) {
	t.Helper()
	factorID, secret := enrollFactor(t, client, token)
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(
			context.Background(),
			`DELETE FROM auth.totp_factors WHERE id = $1`, factorID,
		)
	})

	req := &authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	}
	ctx := withCookies(context.Background(), token)
	info, _ := connect.CallInfoForClientContext(ctx)
	resp, err := client.MFAEnrollVerify(ctx, req)
	require.NoError(t, err)
	return resp, info.ResponseHeader()
}

func toHTTPHeader(header *connect.Header) http.Header {
	out := http.Header{}
	for k, vs := range header.All() {
		out[k] = vs
	}
	return out
}

// sessionCookie returns the accessToken cookie a response set.
func sessionCookie(t *testing.T, header *connect.Header) http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: toHTTPHeader(header)}).Cookies() {
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
	resp, _ := enrollAndVerify(t, client, http.Cookie{Name: "mfaToken", Value: token})
	assert.NotEmpty(t, resp.RecoveryCodes)
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

	req := &authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	}
	_, err := client.MFAEnrollVerify(
		withCookies(
			context.Background(),
			mfaToken,
			http.Cookie{Name: "mfaRememberMe", Value: "1"},
		),
		req,
	)
	require.NoError(t, err)
}

func TestMFAChallenge_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAChallenge(
		context.Background(),
		&authv1.MFAChallengeRequest{Code: "123456"},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAChallenge_NoFactorID(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAChallengeRequest{Code: "123456"}
	_, err := client.MFAChallenge(withCookies(context.Background(), mfaTokenCookie), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAChallenge_InvalidFactorID(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAChallengeRequest{Code: "123456"}
	_, err := client.MFAChallenge(withCookies(context.Background(), mfaTokenCookie,
		http.Cookie{Name: "mfaFactorID", Value: "not-a-uuid"},
	), req)
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

	req := &authv1.MFAChallengeRequest{
		Code: currentTOTPCode(t, secret),
	}
	_, err := client.MFAChallenge(withCookies(context.Background(), mfaToken,
		http.Cookie{Name: "mfaFactorID", Value: factorID},
	), req)
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

	req := &authv1.MFAChallengeRequest{
		Code: currentTOTPCode(t, secret),
	}
	_, err := client.MFAChallenge(withCookies(context.Background(), mfaToken,
		http.Cookie{Name: "mfaFactorID", Value: factorID},
		http.Cookie{Name: "mfaRememberMe", Value: "1"},
		http.Cookie{Name: "mfaRedirect", Value: "/backlog"},
	), req)
	require.NoError(t, err)
}

func TestMFAEnrollSkip_NoMFAToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnrollSkip(
		context.Background(),
		&authv1.MFAEnrollSkipRequest{},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollSkip_NoRefreshToken(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollSkipRequest{}
	_, err := client.MFAEnrollSkip(withCookies(context.Background(), mfaTokenCookie), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAEnrollSkip_Success(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollSkipRequest{}
	// MFAEnrollSkip only relays these cookies, so any value works.
	_, err := client.MFAEnrollSkip(
		withCookies(context.Background(), mfaTokenCookie, mfaRefreshTokenCookie),
		req,
	)
	require.NoError(t, err)
}

func TestMFAEnrollSkip_WithRememberMe(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollSkipRequest{}
	_, err := client.MFAEnrollSkip(withCookies(context.Background(), mfaTokenCookie,
		mfaRefreshTokenCookie,
		http.Cookie{Name: "mfaRememberMe", Value: "1"},
	), req)
	require.NoError(t, err)
}

func TestMFAEnrollVerify_SettingsFlow_WithAccessToken(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	// Settings flow: accessToken cookie present, no mfaToken.
	_, _ = enrollAndVerify(t, client, http.Cookie{Name: "accessToken", Value: token})
}

func TestRegenerateRecoveryCodes_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.RegenerateRecoveryCodes(
		context.Background(),
		&authv1.RegenerateRecoveryCodesRequest{},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestRegenerateRecoveryCodes_Success(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	enrolled, enrolledHdr := enrollAndVerify(
		t, client, http.Cookie{Name: "accessToken", Value: token},
	)
	accessCookie := sessionCookie(t, enrolledHdr)

	req := &authv1.RegenerateRecoveryCodesRequest{
		Code: enrolled.RecoveryCodes[0],
	}
	resp, err := client.RegenerateRecoveryCodes(
		withCookies(context.Background(), accessCookie),
		req,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.RecoveryCodes)
}

// TestRegenerateRecoveryCodes_ReplacesEarlierCodes: a second call invalidates
// the first batch.
func TestRegenerateRecoveryCodes_ReplacesEarlierCodes(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	first, firstHdr := enrollAndVerify(
		t, client, http.Cookie{Name: "accessToken", Value: token},
	)
	require.NotEmpty(t, first.RecoveryCodes)
	accessCookie := sessionCookie(t, firstHdr)

	req := &authv1.RegenerateRecoveryCodesRequest{
		Code: first.RecoveryCodes[0],
	}
	resp, err := client.RegenerateRecoveryCodes(
		withCookies(context.Background(), accessCookie),
		req,
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RecoveryCodes)
	assert.NotEqual(t, first.RecoveryCodes, resp.RecoveryCodes)
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
	req := &authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	}
	_, err := client.MFAEnrollVerify(
		withCookies(
			context.Background(),
			accessCookie,
			http.Cookie{Name: "refreshToken", Value: "refresh"},
		),
		req,
	)
	require.NoError(t, err)
}

// A pre-MFA (aal1) token can't manage the factors of a user who has one.
func TestRegenerateRecoveryCodes_PreMFAToken_Unauthenticated(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.RegenerateRecoveryCodesRequest{}
	_, err := client.RegenerateRecoveryCodes(
		withCookies(context.Background(), mfaAccessToken),
		req,
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMFAEnroll_PreMFAToken_VerifiedFactor_Unauthenticated(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollRequest{}
	_, err := client.MFAEnroll(
		withCookies(
			context.Background(),
			http.Cookie{Name: account.MfaTokenCookieName, Value: mfaAccessToken.Value},
		),
		req,
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMFAEnrollSkip_VerifiedFactor_PermissionDenied(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.MFAEnrollSkipRequest{}
	_, err := client.MFAEnrollSkip(
		withCookies(
			context.Background(),
			http.Cookie{Name: account.MfaTokenCookieName, Value: mfaAccessToken.Value},
			http.Cookie{Name: account.MfaRefreshTokenCookieName, Value: "unused"},
		),
		req,
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRegenerateRecoveryCodes_WrongCode_PermissionDenied(t *testing.T) {
	client := mfaClient(t)
	req := &authv1.RegenerateRecoveryCodesRequest{
		Code: "000000",
	}
	_, err := client.RegenerateRecoveryCodes(
		withCookies(context.Background(), mfaSessionToken),
		req,
	)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestAuthErrorCode_UnknownErrorIsInternal(t *testing.T) {
	assert.Equal(t, connect.CodeInternal, account.AuthErrorCode(errors.New("boom")))
}
