package agentaccounts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

func userCtx(role models.Role, channel string) context.Context {
	ctx := context.WithValue(context.Background(), constants.UserContextKey,
		models.User{
			ID: "u1", Email: "u@example.com", Role: role,
			AppAccess: nil, HasMFA: false, DisplayName: "",
		})
	if channel != "" {
		ctx = context.WithValue(ctx, constants.AuthChannelContextKey, channel)
	}
	return ctx
}

func TestAdminFromNeedsAnAdminInABrowser(t *testing.T) {
	user, err := adminFrom(userCtx(models.RoleAdmin, constants.ChannelBrowser))
	require.NoError(t, err)
	assert.Equal(t, "u1", user.ID)

	for name, tc := range map[string]struct {
		ctx  context.Context
		want connect.Code
	}{
		"no user": {context.Background(), connect.CodeUnauthenticated},
		"plain user": {
			userCtx(models.RoleUser, constants.ChannelBrowser),
			connect.CodePermissionDenied,
		},
		"agent": {
			userCtx(models.RoleAgent, constants.ChannelBrowser),
			connect.CodePermissionDenied,
		},
		"bearer admin": {
			userCtx(models.RoleAdmin, constants.ChannelMCP),
			connect.CodePermissionDenied,
		},
		"no channel": {userCtx(models.RoleAdmin, ""), connect.CodePermissionDenied},
	} {
		_, err = adminFrom(tc.ctx)
		assert.Equal(t, tc.want, connect.CodeOf(err), name)
	}
}

func TestValidName(t *testing.T) {
	got, ok := validName("  Overnight agent ")
	assert.True(t, ok)
	assert.Equal(t, "Overnight agent", got)

	_, ok = validName(strings.Repeat("a", maxNameLen))
	assert.True(t, ok)
	_, ok = validName(strings.Repeat("é", maxNameLen))
	assert.True(t, ok, "the limit counts characters, not bytes")
	for _, bad := range []string{
		"", " \t ", strings.Repeat("a", maxNameLen+1), "line\nbreak", "tab\there", "nul\x00",
	} {
		_, ok = validName(bad)
		assert.False(t, ok, bad)
	}
}

func TestStepUpCode(t *testing.T) {
	assert.Equal(t, connect.CodeResourceExhausted, stepUpCode(auth.ErrTooManyAttempts))
	assert.Equal(t, connect.CodePermissionDenied, stepUpCode(auth.ErrWrongCredential))
	assert.Equal(t, connect.CodeInternal, stepUpCode(errors.New("db down")))
}

// stepUpReturning is a step-up check that fails with err (nil passes).
func stepUpReturning(err error) *fakeStepUp {
	return &fakeStepUp{err: err, token: "", code: "", calls: 0}
}

type fakeStepUp struct {
	err         error
	token, code string
	calls       int
}

func (f *fakeStepUp) VerifyCurrentFactor(_ context.Context, token, code string) error {
	f.calls++
	f.token, f.code = token, code
	return f.err
}

// agentClient mounts the service for an admin authenticated through channel,
// with a fake step-up check.
func agentClient(
	t *testing.T, channel string, stepUp StepUp,
) factoryv1connect.AgentAccountServiceClient {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	mux := http.NewServeMux()
	Register(mux, logging.NewNopLogger(),
		func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				next(w, r.WithContext(userCtx(models.RoleAdmin, channel)))
			}
		},
		&Deps{Store: NewStore(pool, nil), StepUp: stepUp},
	)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return factoryv1connect.NewAgentAccountServiceClient(connect.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))
}

// withToken sends the session cookie the step-up check reads.
func withToken(ctx context.Context) context.Context {
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", "accessToken=session-jwt")
	return ctx
}

func TestCreateChecksTheStepUpWithTheSessionToken(t *testing.T) {
	stepUp := stepUpReturning(nil)
	c := agentClient(t, constants.ChannelBrowser, stepUp)
	ctx := withToken(context.Background())

	resp, err := c.CreateAgentAccount(ctx, &factoryv1.CreateAgentAccountRequest{
		Name: " Reviewer ", TotpCode: "123456",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, stepUp.calls)
	assert.Equal(t, "session-jwt", stepUp.token)
	assert.Equal(t, "123456", stepUp.code)
	assert.Equal(t, "Reviewer", resp.Account.Name)
	assert.Equal(t, "human:u1", resp.Account.CreatedBy)
	assert.Empty(t, resp.Account.RevokedAt)
	assert.NotEmpty(t, resp.Account.CreatedAt)
	assert.NotEmpty(t, resp.ClientSecret)
}

func TestCreateRefusesWithoutAPassingStepUp(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want connect.Code
	}{
		"wrong code": {auth.ErrWrongCredential, connect.CodePermissionDenied},
		"locked out": {auth.ErrTooManyAttempts, connect.CodeResourceExhausted},
		"other":      {errors.New("db down"), connect.CodeInternal},
	} {
		stepUp := stepUpReturning(tc.err)
		c := agentClient(t, constants.ChannelBrowser, stepUp)
		_, err := c.CreateAgentAccount(withToken(context.Background()),
			&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: "1"})
		assert.Equal(t, tc.want, connect.CodeOf(err), name)
	}
}

func TestCreateNeedsASessionCookieAndAValidName(t *testing.T) {
	stepUp := stepUpReturning(nil)
	c := agentClient(t, constants.ChannelBrowser, stepUp)

	_, err := c.CreateAgentAccount(context.Background(),
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: "1"})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = c.CreateAgentAccount(withToken(context.Background()),
		&factoryv1.CreateAgentAccountRequest{Name: " ", TotpCode: "1"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Zero(t, stepUp.calls, "a bad name is refused before any code is spent")
}

func TestListAndRevokeThroughTheService(t *testing.T) {
	c := agentClient(t, constants.ChannelBrowser, stepUpReturning(nil))
	created, err := c.CreateAgentAccount(withToken(context.Background()),
		&factoryv1.CreateAgentAccountRequest{Name: "listed", TotpCode: "1"})
	require.NoError(t, err)
	id := created.Account.Id

	list, err := c.ListAgentAccounts(context.Background(),
		&factoryv1.ListAgentAccountsRequest{})
	require.NoError(t, err)
	var found *factoryv1.AgentAccount
	for _, a := range list.Accounts {
		if a.Id == id {
			found = a
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, created.Account.ClientId, found.ClientId)
	assert.Equal(t, created.Account.UserId, found.UserId)
	assert.Empty(t, found.RevokedAt)

	revoked, err := c.RevokeAgentAccount(context.Background(),
		&factoryv1.RevokeAgentAccountRequest{Id: id})
	require.NoError(t, err)
	assert.NotEmpty(t, revoked.Account.RevokedAt)
}

func TestEveryRPCNeedsAnAdminInABrowser(t *testing.T) {
	c := agentClient(t, constants.ChannelMCP, stepUpReturning(nil))
	ctx := withToken(context.Background())

	_, err := c.ListAgentAccounts(ctx, &factoryv1.ListAgentAccountsRequest{})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = c.CreateAgentAccount(ctx,
		&factoryv1.CreateAgentAccountRequest{Name: "x", TotpCode: "1"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = c.RevokeAgentAccount(ctx, &factoryv1.RevokeAgentAccountRequest{Id: "x"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRevokeRejectsABadOrUnknownID(t *testing.T) {
	c := agentClient(t, constants.ChannelBrowser, stepUpReturning(nil))

	_, err := c.RevokeAgentAccount(context.Background(),
		&factoryv1.RevokeAgentAccountRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = c.RevokeAgentAccount(context.Background(),
		&factoryv1.RevokeAgentAccountRequest{Id: "00000000-0000-0000-0000-000000000000"})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
