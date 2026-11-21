//nolint:exhaustruct // fakes and fixtures set only the fields a test reads
package connect

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	rpc "connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/family"
	"tools.xdoubleu.com/internal/platform/identity/adapters"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

func userCtx(role models.Role, channel string) context.Context {
	ctx := context.WithValue(context.Background(), constants.UserContextKey,
		models.User{
			ID: "u1", Email: "u@example.com", Role: role,
			AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false, DisplayName: "",
		})
	if channel != "" {
		ctx = context.WithValue(ctx, constants.AuthChannelContextKey, channel)
	}
	return ctx
}

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
	t *testing.T, channel string, stepUp adapters.FactorChecker,
) factoryv1connect.AgentAccountServiceClient {
	t.Helper()
	return agentClientNotifying(t, channel, stepUp, nil)
}

// agentClientNotifying is agentClient with a Notify hook.
func agentClientNotifying(
	t *testing.T,
	channel string,
	stepUp adapters.FactorChecker,
	notify func(context.Context, string),
) factoryv1connect.AgentAccountServiceClient {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	var notifier agentaccounts.Notifier
	if notify != nil {
		notifier = adapters.SlackNotifier{Send: notify}
	}
	mux := http.NewServeMux()
	Register(mux, logging.NewNopLogger(),
		func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				next(w, r.WithContext(userCtx(models.RoleAdmin, channel)))
			}
		},
		agentaccounts.NewService(
			agentaccounts.NewStore(pool, nil, mcptools.ObservabilityGrant),
			adapters.StepUp{Auth: stepUp},
			family.NewFamilyRepository(pool),
			notifier,
		),
	)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return factoryv1connect.NewAgentAccountServiceClient(rpc.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))
}

// withToken sends the session cookie the step-up check reads.
func withToken(ctx context.Context) context.Context {
	ctx, info := rpc.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", "accessToken=session-jwt")
	return ctx
}

func TestCreateNeedsNoStepUp(t *testing.T) {
	stepUp := stepUpReturning(nil)
	c := agentClient(t, constants.ChannelBrowser, stepUp)

	resp, err := c.CreateAgentAccount(context.Background(),
		&factoryv1.CreateAgentAccountRequest{Name: " Reviewer "})
	require.NoError(t, err)

	assert.Zero(t, stepUp.calls)
	assert.Equal(t, "Reviewer", resp.Account.Name)
	assert.Equal(t, "human:u1", resp.Account.CreatedBy)
	assert.Empty(t, resp.Account.RevokedAt)
	assert.NotEmpty(t, resp.Account.CreatedAt)
	assert.NotEmpty(t, resp.ClientSecret)
}

func TestCreatingAnAccountIsPostedWithItsEscapedName(t *testing.T) {
	var posted []string
	c := agentClientNotifying(t, constants.ChannelBrowser, stepUpReturning(nil),
		func(_ context.Context, text string) { posted = append(posted, text) })

	_, err := c.CreateAgentAccount(withToken(context.Background()),
		&factoryv1.CreateAgentAccountRequest{Name: "<!channel> bot"})
	require.NoError(t, err)

	assert.Equal(t,
		[]string{":key: An agent account was created: &lt;!channel&gt; bot"}, posted)
}

func TestCreateRefusesAnInvalidName(t *testing.T) {
	c := agentClient(t, constants.ChannelBrowser, stepUpReturning(nil))

	_, err := c.CreateAgentAccount(context.Background(),
		&factoryv1.CreateAgentAccountRequest{Name: " "})
	assert.Equal(t, rpc.CodeInvalidArgument, rpc.CodeOf(err))
}

func TestListAndRevokeThroughTheService(t *testing.T) {
	c := agentClient(t, constants.ChannelBrowser, stepUpReturning(nil))
	created, err := c.CreateAgentAccount(withToken(context.Background()),
		&factoryv1.CreateAgentAccountRequest{Name: "listed"})
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
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
	_, err = c.CreateAgentAccount(ctx,
		&factoryv1.CreateAgentAccountRequest{Name: "x"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
	_, err = c.RevokeAgentAccount(ctx, &factoryv1.RevokeAgentAccountRequest{Id: "x"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
	_, err = c.RotateAgentAccountSecret(ctx,
		&factoryv1.RotateAgentAccountSecretRequest{Id: "x", TotpCode: "1"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
	_, err = c.AddAgentAccountToFamily(ctx,
		&factoryv1.AddAgentAccountToFamilyRequest{Id: "x"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
	_, err = c.RemoveAgentAccountFromFamily(ctx,
		&factoryv1.RemoveAgentAccountFromFamilyRequest{Id: "x"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
}

func TestAnAdminAddsAndRemovesAnAgentInTheirFamily(t *testing.T) {
	stepUp := stepUpReturning(nil)
	c := agentClient(t, constants.ChannelBrowser, stepUp)
	ctx := withToken(context.Background())
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM global.families WHERE id IN (
				SELECT family_id FROM global.family_members WHERE user_id = 'u1')`)
	})
	created, err := c.CreateAgentAccount(ctx,
		&factoryv1.CreateAgentAccountRequest{Name: "family bot"})
	require.NoError(t, err)

	added, err := c.AddAgentAccountToFamily(ctx,
		&factoryv1.AddAgentAccountToFamilyRequest{
			Id: created.Account.Id, TotpCode: "424242",
		})
	require.NoError(t, err)
	assert.Equal(t, created.Account.Id, added.Account.Id)
	assert.Equal(t, "424242", stepUp.code)

	_, err = c.AddAgentAccountToFamily(ctx,
		&factoryv1.AddAgentAccountToFamilyRequest{Id: "nope", TotpCode: "1"})
	assert.Equal(t, rpc.CodeInvalidArgument, rpc.CodeOf(err))
	_, err = c.AddAgentAccountToFamily(context.Background(),
		&factoryv1.AddAgentAccountToFamilyRequest{Id: created.Account.Id})
	assert.Equal(t, rpc.CodeUnauthenticated, rpc.CodeOf(err))
	denied := agentClient(t, constants.ChannelBrowser,
		stepUpReturning(auth.ErrWrongCredential))
	_, err = denied.AddAgentAccountToFamily(ctx,
		&factoryv1.AddAgentAccountToFamilyRequest{Id: created.Account.Id})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))

	removed, err := c.RemoveAgentAccountFromFamily(ctx,
		&factoryv1.RemoveAgentAccountFromFamilyRequest{Id: created.Account.Id})
	require.NoError(t, err)
	assert.Equal(t, created.Account.Id, removed.Account.Id)
	_, err = c.RemoveAgentAccountFromFamily(ctx,
		&factoryv1.RemoveAgentAccountFromFamilyRequest{Id: "nope"})
	assert.Equal(t, rpc.CodeInvalidArgument, rpc.CodeOf(err))
}

func TestRotatingASecretNeedsTheStepUpAndReturnsANewOne(t *testing.T) {
	stepUp := stepUpReturning(nil)
	c := agentClient(t, constants.ChannelBrowser, stepUp)
	ctx := withToken(context.Background())
	created, err := c.CreateAgentAccount(ctx,
		&factoryv1.CreateAgentAccountRequest{Name: "rotating"})
	require.NoError(t, err)

	rotated, err := c.RotateAgentAccountSecret(ctx,
		&factoryv1.RotateAgentAccountSecretRequest{
			Id: created.Account.Id, TotpCode: "654321",
		})
	require.NoError(t, err)
	assert.Equal(t, created.Account.Id, rotated.Account.Id)
	assert.NotEmpty(t, rotated.ClientSecret)
	assert.NotEqual(t, created.ClientSecret, rotated.ClientSecret)
	assert.Equal(t, "654321", stepUp.code)

	_, err = c.RotateAgentAccountSecret(ctx,
		&factoryv1.RotateAgentAccountSecretRequest{Id: "nope", TotpCode: "1"})
	assert.Equal(t, rpc.CodeInvalidArgument, rpc.CodeOf(err))
	_, err = c.RotateAgentAccountSecret(context.Background(),
		&factoryv1.RotateAgentAccountSecretRequest{Id: created.Account.Id, TotpCode: "1"})
	assert.Equal(t, rpc.CodeUnauthenticated, rpc.CodeOf(err))

	failing := agentClient(t, constants.ChannelBrowser,
		stepUpReturning(auth.ErrWrongCredential))
	_, err = failing.RotateAgentAccountSecret(ctx,
		&factoryv1.RotateAgentAccountSecretRequest{Id: created.Account.Id, TotpCode: "1"})
	assert.Equal(t, rpc.CodePermissionDenied, rpc.CodeOf(err))
}

func TestRevokeRejectsABadOrUnknownID(t *testing.T) {
	c := agentClient(t, constants.ChannelBrowser, stepUpReturning(nil))

	_, err := c.RevokeAgentAccount(context.Background(),
		&factoryv1.RevokeAgentAccountRequest{Id: "nope"})
	assert.Equal(t, rpc.CodeInvalidArgument, rpc.CodeOf(err))
	_, err = c.RevokeAgentAccount(context.Background(),
		&factoryv1.RevokeAgentAccountRequest{Id: "00000000-0000-0000-0000-000000000000"})
	assert.Equal(t, rpc.CodeNotFound, rpc.CodeOf(err))
}

func TestCallerFromReadsTheUserChannelAndSession(t *testing.T) {
	assert.Equal(t, agentaccounts.Caller{}, callerFrom(context.Background()))

	got := callerFrom(userCtx(models.RoleAdmin, constants.ChannelBrowser))
	assert.Equal(t, "u1", got.UserID)
	assert.True(t, got.Admin)
	assert.True(t, got.Browser)

	got = callerFrom(userCtx(models.RoleAgent, constants.ChannelMCP))
	assert.False(t, got.Admin)
	assert.False(t, got.Browser)

	assert.False(t, callerFrom(userCtx(models.RoleAdmin, "")).Browser)
}

func TestMapErrorGivesEachRefusalItsCode(t *testing.T) {
	cases := []struct {
		err  error
		want rpc.Code
	}{
		{agentaccounts.ErrUnauthenticated, rpc.CodeUnauthenticated},
		{agentaccounts.ErrNotSignedIn, rpc.CodeUnauthenticated},
		{agentaccounts.ErrNotAdmin, rpc.CodePermissionDenied},
		{agentaccounts.ErrNotBrowser, rpc.CodePermissionDenied},
		{agentaccounts.ErrWrongCredential, rpc.CodePermissionDenied},
		{agentaccounts.ErrInvalidName, rpc.CodeInvalidArgument},
		{agentaccounts.ErrInvalidID, rpc.CodeInvalidArgument},
		{agentaccounts.ErrTooManyAttempts, rpc.CodeResourceExhausted},
		{errors.New("db down"), rpc.CodeInternal},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, rpc.CodeOf(mapError(tc.err)), tc.err.Error())
	}
}
