package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/internal/mocks"
)

func mfaClient(t *testing.T) authv1connect.AuthServiceClient {
	t.Helper()
	ts := connectServer(t)
	return authv1connect.NewAuthServiceClient(ts.Client(), ts.URL)
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
	req := connect.NewRequest(&authv1.MFAEnrollRequest{})
	setCookieOnRequest(req, mfaTokenCookie)
	resp, err := client.MFAEnroll(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.FactorId)
}

func TestMFAEnrollVerify_NoToken(t *testing.T) {
	client := mfaClient(t)
	_, err := client.MFAEnrollVerify(context.Background(), connect.NewRequest(
		&authv1.MFAEnrollVerifyRequest{
			FactorId: mocks.MockedFactorID.String(),
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

func TestMFAEnrollVerify_Success(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: mocks.MockedFactorID.String(),
		Code:     "123456",
	})
	setCookieOnRequest(req, mfaTokenCookie)
	_, err := client.MFAEnrollVerify(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAEnrollVerify_WithRememberMe(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: mocks.MockedFactorID.String(),
		Code:     "123456",
	})
	setCookieOnRequest(
		req,
		mfaTokenCookie,
		http.Cookie{Name: "mfaRememberMe", Value: "1"},
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
	req := connect.NewRequest(&authv1.MFAChallengeRequest{Code: "123456"})
	setCookieOnRequest(req,
		mfaTokenCookie,
		http.Cookie{Name: "mfaFactorID", Value: mocks.MockedFactorID.String()},
	)
	_, err := client.MFAChallenge(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAChallenge_WithRememberMeAndRedirect(t *testing.T) {
	client := mfaClient(t)
	req := connect.NewRequest(&authv1.MFAChallengeRequest{Code: "123456"})
	setCookieOnRequest(req,
		mfaTokenCookie,
		http.Cookie{Name: "mfaFactorID", Value: mocks.MockedFactorID.String()},
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
