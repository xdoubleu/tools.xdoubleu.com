//nolint:exhaustruct // fakes and fixtures set only the fields a test reads
package oauth2as_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/identity/oauth2as"
)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

type fakeAgents struct {
	agentUserID string
	err         error
	undelegErr  error
	delegated   []string
	undelegated []string
}

func (f *fakeAgents) Delegate(
	_ context.Context, accountID, clientID, delegatedBy string,
) (string, error) {
	f.delegated = append(f.delegated, accountID+"|"+clientID+"|"+delegatedBy)
	return f.agentUserID, f.err
}

func (f *fakeAgents) Undelegate(_ context.Context, clientID string) error {
	f.undelegated = append(f.undelegated, clientID)
	return f.undelegErr
}

type fakeUsers struct{ err error }

func (f fakeUsers) Profile(
	_ context.Context,
	id string,
) (oauth2as.ResolvedUser, error) {
	return oauth2as.ResolvedUser{
		ID: id, Email: "agent@agent.invalid", DisplayName: "Agent", IsAdmin: true,
	}, f.err
}

type fakeGrants struct {
	covered     bool
	coversErr   error
	rememberErr error
	remembered  []string
}

func (f *fakeGrants) Covers(context.Context, string, string, []string) (bool, error) {
	return f.covered, f.coversErr
}

func (f *fakeGrants) Remember(
	_ context.Context,
	userID, clientID string,
	_ []string,
) error {
	f.remembered = append(f.remembered, userID+"|"+clientID)
	return f.rememberErr
}

func adminUser() oauth2as.ResolvedUser {
	return oauth2as.ResolvedUser{
		ID: "admin-1", Email: "a@example.com", DisplayName: "Admin", IsAdmin: true,
	}
}

func personUser() oauth2as.ResolvedUser {
	return oauth2as.ResolvedUser{
		ID: "person-1", Email: "p@example.com", DisplayName: "Person", IsAdmin: false,
	}
}

func newAuthorizer(
	agents oauth2as.AgentAccounts, grants oauth2as.ConsentGrants, clock oauth2as.Clock,
) *oauth2as.Authorizer {
	return oauth2as.NewAuthorizer(
		agents, fakeUsers{}, grants, "secret", clock, logging.NewNopLogger(),
	)
}

func TestSubjectIsTheApprovingUserAndEndsAnyDelegation(t *testing.T) {
	agents := &fakeAgents{}
	authz := newAuthorizer(agents, nil, oauth2as.SystemClock{})

	got, err := authz.Subject(context.Background(), personUser(), "client-1", "")

	require.NoError(t, err)
	assert.Equal(t, personUser(), got)
	assert.Equal(t, []string{"client-1"}, agents.undelegated)
	assert.Empty(t, agents.delegated)
}

func TestSubjectWithoutAgentAccountsIsTheUser(t *testing.T) {
	authz := newAuthorizer(nil, nil, oauth2as.SystemClock{})

	got, err := authz.Subject(context.Background(), personUser(), "client-1", "")

	require.NoError(t, err)
	assert.Equal(t, personUser(), got)
}

func TestSubjectFailsWhenTheDelegationCantBeEnded(t *testing.T) {
	cause := errors.New("db down")
	authz := newAuthorizer(&fakeAgents{undelegErr: cause}, nil, oauth2as.SystemClock{})

	_, err := authz.Subject(context.Background(), personUser(), "client-1", "")

	assert.ErrorIs(t, err, oauth2as.ErrUndelegationFailed)
	assert.ErrorIs(t, err, cause)
}

func TestAnAdminDelegatesToTheAgentAccountsUser(t *testing.T) {
	agents := &fakeAgents{agentUserID: "agent-user"}
	authz := newAuthorizer(agents, nil, oauth2as.SystemClock{})

	got, err := authz.Subject(context.Background(), adminUser(), "client-1", "account-1")

	require.NoError(t, err)
	assert.Equal(t, "agent-user", got.ID)
	assert.Equal(t, "agent@agent.invalid", got.Email)
	assert.False(t, got.IsAdmin, "the agent never inherits the admin role")
	assert.Equal(t, []string{"account-1|client-1|human:admin-1"}, agents.delegated)
	assert.Empty(t, agents.undelegated)
}

func TestOnlyAnAdminDelegates(t *testing.T) {
	agents := &fakeAgents{agentUserID: "agent-user"}

	_, err := newAuthorizer(agents, nil, oauth2as.SystemClock{}).
		Subject(context.Background(), personUser(), "client-1", "account-1")
	assert.ErrorIs(t, err, oauth2as.ErrDelegationForbidden)
	assert.Empty(t, agents.delegated)

	_, err = newAuthorizer(nil, nil, oauth2as.SystemClock{}).
		Subject(context.Background(), adminUser(), "client-1", "account-1")
	assert.ErrorIs(t, err, oauth2as.ErrDelegationForbidden, "no agent accounts configured")
}

func TestADelegationThatCantBeMadeIsRefused(t *testing.T) {
	cause := errors.New("revoked")
	authz := newAuthorizer(&fakeAgents{err: cause}, nil, oauth2as.SystemClock{})
	_, err := authz.Subject(context.Background(), adminUser(), "client-1", "account-1")
	assert.ErrorIs(t, err, oauth2as.ErrDelegationFailed)
	assert.ErrorIs(t, err, cause)

	profileErr := errors.New("no such user")
	authz = oauth2as.NewAuthorizer(
		&fakeAgents{}, fakeUsers{err: profileErr}, nil, "secret",
		oauth2as.SystemClock{}, logging.NewNopLogger(),
	)
	_, err = authz.Subject(context.Background(), adminUser(), "client-1", "account-1")
	assert.ErrorIs(t, err, oauth2as.ErrDelegationFailed)
	assert.ErrorIs(t, err, profileErr)
}

func TestConsentTokenIsBoundToUserRequestAndTime(t *testing.T) {
	clock := &fixedClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	authz := newAuthorizer(nil, nil, clock)
	q := url.Values{"client_id": {"c1"}, "state": {"s"}}

	token := authz.NewConsentToken("u1", q)

	assert.True(t, authz.VerifyConsent("u1", q, token))
	assert.False(t, authz.VerifyConsent("u2", q, token), "another user")
	assert.False(t, authz.VerifyConsent(
		"u1", url.Values{"client_id": {"c1"}, "state": {"other"}}, token,
	), "another request")

	clock.now = clock.now.Add(9 * time.Minute)
	assert.True(t, authz.VerifyConsent("u1", q, token))
	clock.now = clock.now.Add(2 * time.Minute)
	assert.False(t, authz.VerifyConsent("u1", q, token), "expired")
}

func TestConsentTokenNeedsTheServersSecret(t *testing.T) {
	clock := &fixedClock{now: time.Now()}
	other := oauth2as.NewAuthorizer(
		nil, fakeUsers{}, nil, "other-secret", clock, logging.NewNopLogger(),
	)
	q := url.Values{"client_id": {"c1"}}

	assert.False(t, newAuthorizer(nil, nil, clock).
		VerifyConsent("u1", q, other.NewConsentToken("u1", q)))
}

func TestRememberedNeedsACoveringGrantAndNoDelegation(t *testing.T) {
	ctx := context.Background()
	scopes := []string{"openid"}

	covered := newAuthorizer(nil, &fakeGrants{covered: true}, oauth2as.SystemClock{})
	assert.True(t, covered.Remembered(ctx, "u1", "c1", scopes, ""))
	assert.False(t, covered.Remembered(ctx, "u1", "c1", scopes, "account-1"),
		"a delegation is always decided afresh")

	uncovered := newAuthorizer(nil, &fakeGrants{covered: false}, oauth2as.SystemClock{})
	assert.False(t, uncovered.Remembered(ctx, "u1", "c1", scopes, ""))

	none := newAuthorizer(nil, nil, oauth2as.SystemClock{})
	assert.False(t, none.Remembered(ctx, "u1", "c1", scopes, ""))
}

func TestAFailedGrantLookupMeansConsentShowsAgain(t *testing.T) {
	grants := &fakeGrants{covered: true, coversErr: errors.New("db down")}

	got := newAuthorizer(nil, grants, oauth2as.SystemClock{}).
		Remembered(context.Background(), "u1", "c1", []string{"openid"}, "")

	assert.False(t, got)
}

func TestRememberStoresOnlyAPersonsOwnApproval(t *testing.T) {
	ctx := context.Background()
	grants := &fakeGrants{}
	authz := newAuthorizer(nil, grants, oauth2as.SystemClock{})

	authz.Remember(ctx, "u1", "c1", []string{"openid"}, "account-1")
	assert.Empty(t, grants.remembered, "a delegation is not remembered")

	authz.Remember(ctx, "u1", "c1", []string{"openid"}, "")
	assert.Equal(t, []string{"u1|c1"}, grants.remembered)
}

func TestRememberIsBestEffort(t *testing.T) {
	grants := &fakeGrants{rememberErr: errors.New("db down")}

	assert.NotPanics(t, func() {
		newAuthorizer(nil, grants, oauth2as.SystemClock{}).
			Remember(context.Background(), "u1", "c1", []string{"openid"}, "")
		newAuthorizer(nil, nil, oauth2as.SystemClock{}).
			Remember(context.Background(), "u1", "c1", []string{"openid"}, "")
	})
}
