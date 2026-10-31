package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	authv1 "tools.xdoubleu.com/gen/auth/v1"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/models"
)

// agentAdmin is a signed-in admin with a verified TOTP factor, so it can pass
// the step-up check with a recovery code.
type agentAdmin struct {
	userID  string
	session http.Cookie
	// recoveryCodes are single use: each step-up consumes one.
	recoveryCodes []string
}

func newAgentAdmin(t *testing.T, role models.Role) *agentAdmin {
	t.Helper()
	ctx := context.Background()
	token := freshTestUser(t)
	tokenCookie := http.Cookie{Name: "accessToken", Value: token}

	user, err := testApp.auth.ResolveToken(ctx, token)
	require.NoError(t, err)
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, user.ID, user.Email))
	require.NoError(t, testApp.appUsersRepo.SetRole(ctx, user.ID, role))
	testApp.auth.InvalidateUserCache()

	client := mfaClient(t)
	factorID, secret := enrollFactor(t, client, tokenCookie)
	verifyCtx := withCookies(ctx, tokenCookie)
	info, _ := connect.CallInfoForClientContext(verifyCtx)
	resp, err := client.MFAEnrollVerify(verifyCtx, &authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	})
	require.NoError(t, err)

	return &agentAdmin{
		userID:        user.ID,
		session:       sessionCookie(t, info.ResponseHeader()),
		recoveryCodes: resp.RecoveryCodes,
	}
}

func (a *agentAdmin) nextCode() string {
	code := a.recoveryCodes[0]
	a.recoveryCodes = a.recoveryCodes[1:]
	return code
}

func (a *agentAdmin) ctx() context.Context {
	return withCookies(context.Background(), a.session)
}

func agentAccountClient(t *testing.T) factoryv1connect.AgentAccountServiceClient {
	t.Helper()
	ts := connectServer(t)
	return factoryv1connect.NewAgentAccountServiceClient(connect.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))
}

func createAgent(
	t *testing.T,
	admin *agentAdmin,
	name string,
) *factoryv1.CreateAgentAccountResponse {
	t.Helper()
	resp, err := agentAccountClient(t).CreateAgentAccount(admin.ctx(),
		&factoryv1.CreateAgentAccountRequest{Name: name, TotpCode: admin.nextCode()})
	require.NoError(t, err)
	return resp
}

// agentToken exchanges the account's client credentials for an access token,
// returning the token endpoint's status too.
func agentToken(t *testing.T, clientID, secret string) (string, int) {
	t.Helper()
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		connectServer(t).URL+oauth2TokenPath, strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, secret)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	assert.Empty(t, body.RefreshToken, "agent tokens have no refresh token")
	return body.AccessToken, resp.StatusCode
}

func toolCall(name string) *mcp.CallToolParams {
	return &mcp.CallToolParams{
		Meta:           nil,
		Name:           name,
		Arguments:      nil,
		InputResponses: nil,
		RequestState:   "",
	}
}

func mcpStatus(t *testing.T, token string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, connectServer(t).URL+appsMCPPath, nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestAgentAccount_LifeCycle(t *testing.T) {
	admin := newAgentAdmin(t, models.RoleAdmin)
	client := agentAccountClient(t)

	created := createAgent(t, admin, "  Overnight agent  ")
	account := created.Account
	assert.Equal(t, "Overnight agent", account.Name)
	assert.Equal(t, "human:"+admin.userID, account.CreatedBy)
	assert.Empty(t, account.RevokedAt)
	assert.NotEmpty(t, created.ClientSecret)

	list, err := client.ListAgentAccounts(
		admin.ctx(),
		&factoryv1.ListAgentAccountsRequest{},
	)
	require.NoError(t, err)
	var listed *factoryv1.AgentAccount
	for _, a := range list.Accounts {
		if a.Id == account.Id {
			listed = a
		}
	}
	require.NotNil(t, listed)
	assert.Empty(t, listed.RevokedAt)

	// The agent signs in with its own credentials and acts as its own user.
	token, status := agentToken(t, account.ClientId, created.ClientSecret)
	require.Equal(t, http.StatusOK, status)
	user, err := testApp.auth.ResolveToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, account.UserId, user.ID)
	assert.Equal(t, models.RoleAgent, user.Role)

	_, status = agentToken(t, account.ClientId, "wrong-secret")
	assert.Equal(t, http.StatusUnauthorized, status)

	// It connects to /apps/mcp but sees none of the other tools, and can't call
	// them either.
	session := appsMCPSession(t, token)
	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, tools.Tools)
	for _, name := range []string{"get_job_stats", "books_get_library", "get_logs"} {
		_, callErr := session.CallTool(context.Background(), toolCall(name))
		require.Error(t, callErr, name)
		assert.Contains(t, callErr.Error(), "unknown tool", name)
	}

	// Revoking ends the token at once and refuses a new one.
	revoked, err := client.RevokeAgentAccount(admin.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: account.Id})
	require.NoError(t, err)
	assert.NotEmpty(t, revoked.Account.RevokedAt)
	assert.Equal(t, http.StatusUnauthorized, mcpStatus(t, token))
	_, status = agentToken(t, account.ClientId, created.ClientSecret)
	assert.Equal(t, http.StatusUnauthorized, status)

	again, err := client.RevokeAgentAccount(admin.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: account.Id})
	require.NoError(t, err)
	assert.Equal(t, revoked.Account.RevokedAt, again.Account.RevokedAt)
}

func TestAgentAccount_NeedsTheStepUpCode(t *testing.T) {
	admin := newAgentAdmin(t, models.RoleAdmin)
	client := agentAccountClient(t)

	_, err := client.CreateAgentAccount(admin.ctx(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: "000000"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	_, err = client.CreateAgentAccount(context.Background(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: admin.recoveryCodes[0]})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	// A used recovery code is spent.
	spent := admin.nextCode()
	_, err = client.CreateAgentAccount(admin.ctx(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: spent})
	require.NoError(t, err)
	_, err = client.CreateAgentAccount(admin.ctx(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: spent})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestAgentAccount_OnlyAdminsManageThem(t *testing.T) {
	user := newAgentAdmin(t, models.RoleUser)
	client := agentAccountClient(t)

	_, err := client.ListAgentAccounts(user.ctx(), &factoryv1.ListAgentAccountsRequest{})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = client.CreateAgentAccount(user.ctx(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: user.nextCode()})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = client.RevokeAgentAccount(user.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: "00000000-0000-0000-0000-000000000000"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestAgentAccount_InvalidInput(t *testing.T) {
	admin := newAgentAdmin(t, models.RoleAdmin)
	client := agentAccountClient(t)

	for _, name := range []string{"", "   ", strings.Repeat("a", 81), "bad\nname"} {
		_, err := client.CreateAgentAccount(admin.ctx(),
			&factoryv1.CreateAgentAccountRequest{Name: name, TotpCode: admin.nextCode()})
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}

	_, err := client.RevokeAgentAccount(admin.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = client.RevokeAgentAccount(admin.ctx(),
		&factoryv1.RevokeAgentAccountRequest{Id: "00000000-0000-0000-0000-000000000000"})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestAgentUser_CannotBePromoted: an admin can't turn an agent into a person
// with a role or app access, and the user list leaves agents out.
func TestAgentUser_CannotBePromoted(t *testing.T) {
	admin := newAgentAdmin(t, models.RoleAdmin)
	account := createAgent(t, admin, "agent under test").Account
	ts := connectServer(t)
	access := accessv1connect.NewAccessServiceClient(connect.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))

	_, err := access.SetRole(admin.ctx(),
		&accessv1.SetRoleRequest{UserId: account.UserId, Role: "admin"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = access.SetAppAccess(admin.ctx(), &accessv1.SetAppAccessRequest{
		UserId: account.UserId, AppName: "books", Grant: true,
	})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	users, err := access.ListUsers(admin.ctx(), &accessv1.ListUsersRequest{})
	require.NoError(t, err)
	for _, u := range users.Users {
		assert.NotEqual(t, account.UserId, u.Id)
	}

	stored, err := testApp.appUsersRepo.GetByID(context.Background(), account.UserId)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAgent, stored.Role)
	assert.Empty(t, stored.AppAccess)
}
