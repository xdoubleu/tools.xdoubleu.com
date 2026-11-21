//nolint:exhaustruct // fakes and fixtures set only the fields a test reads
package agentaccounts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
)

var errStore = errors.New("store down")

type fakeAccounts struct {
	calls     []string
	createdBy string
	name      string
	err       error
}

func (f *fakeAccounts) account() *agentaccounts.Account {
	return &agentaccounts.Account{
		ID: uuid.New(), Name: f.name, UserID: uuid.New(), ClientID: "agent-1",
		CreatedBy: f.createdBy, CreatedAt: time.Time{}, RevokedAt: nil,
		JobID: nil, TaskID: nil, AppAccess: nil, WriteAccess: nil,
	}
}

func (f *fakeAccounts) List(context.Context) ([]agentaccounts.Account, error) {
	f.calls = append(f.calls, "list")
	return []agentaccounts.Account{*f.account()}, f.err
}

func (f *fakeAccounts) Create(
	_ context.Context, createdBy, name string,
) (*agentaccounts.Account, string, error) {
	f.calls = append(f.calls, "create")
	f.createdBy, f.name = createdBy, name
	return f.account(), "secret", f.err
}

func (f *fakeAccounts) Revoke(
	context.Context,
	uuid.UUID,
) (*agentaccounts.Account, error) {
	f.calls = append(f.calls, "revoke")
	return f.account(), f.err
}

func (f *fakeAccounts) RotateSecret(
	context.Context, uuid.UUID,
) (*agentaccounts.Account, string, error) {
	f.calls = append(f.calls, "rotate")
	return f.account(), "new-secret", f.err
}

func (f *fakeAccounts) JoinFamily(
	_ context.Context, _, _ uuid.UUID,
) (*agentaccounts.Account, error) {
	f.calls = append(f.calls, "join")
	return f.account(), f.err
}

func (f *fakeAccounts) LeaveFamily(
	context.Context,
	uuid.UUID,
) (*agentaccounts.Account, error) {
	f.calls = append(f.calls, "leave")
	return f.account(), f.err
}

type fakeStepUp struct {
	err         error
	token, code string
}

func (f *fakeStepUp) VerifyCurrentFactor(_ context.Context, token, code string) error {
	f.token, f.code = token, code
	return f.err
}

type fakeFamilies struct {
	userID string
	err    error
}

func (f *fakeFamilies) EnsureFamily(
	_ context.Context,
	userID string,
) (uuid.UUID, error) {
	f.userID = userID
	return uuid.New(), f.err
}

type fakeNotifier struct{ events []string }

func (f *fakeNotifier) Created(_ context.Context, name string) {
	f.events = append(f.events, "created:"+name)
}

func (f *fakeNotifier) JoinedFamily(_ context.Context, name string) {
	f.events = append(f.events, "joined:"+name)
}

type fixture struct {
	svc      *agentaccounts.Service
	accounts *fakeAccounts
	stepUp   *fakeStepUp
	families *fakeFamilies
	notify   *fakeNotifier
}

func newFixture() *fixture {
	f := &fixture{
		accounts: &fakeAccounts{}, stepUp: &fakeStepUp{},
		families: &fakeFamilies{}, notify: &fakeNotifier{},
	}
	f.svc = agentaccounts.NewService(f.accounts, f.stepUp, f.families, f.notify)
	return f
}

func adminCaller() agentaccounts.Caller {
	return agentaccounts.Caller{
		UserID: "u1", Admin: true, Browser: true, AccessToken: "session",
	}
}

func TestEveryUseCaseNeedsAnAdminInABrowser(t *testing.T) {
	id := uuid.NewString()
	calls := map[string]func(f *fixture, c agentaccounts.Caller) error{
		"list": func(f *fixture, c agentaccounts.Caller) error {
			_, err := f.svc.List(context.Background(), c)
			return err
		},
		"create": func(f *fixture, c agentaccounts.Caller) error {
			_, _, err := f.svc.Create(context.Background(), c, "bot")
			return err
		},
		"revoke": func(f *fixture, c agentaccounts.Caller) error {
			_, err := f.svc.Revoke(context.Background(), c, id)
			return err
		},
		"rotate": func(f *fixture, c agentaccounts.Caller) error {
			_, _, err := f.svc.RotateSecret(context.Background(), c, id, "1")
			return err
		},
		"add to family": func(f *fixture, c agentaccounts.Caller) error {
			_, err := f.svc.AddToFamily(context.Background(), c, id, "1")
			return err
		},
		"remove from family": func(f *fixture, c agentaccounts.Caller) error {
			_, err := f.svc.RemoveFromFamily(context.Background(), c, id)
			return err
		},
	}
	refused := map[string]struct {
		caller agentaccounts.Caller
		want   error
	}{
		"nobody": {agentaccounts.Caller{}, agentaccounts.ErrUnauthenticated},
		"plain user": {
			agentaccounts.Caller{UserID: "u", Browser: true, AccessToken: "t"},
			agentaccounts.ErrNotAdmin,
		},
		"bearer admin": {
			agentaccounts.Caller{UserID: "u", Admin: true, AccessToken: "t"},
			agentaccounts.ErrNotBrowser,
		},
	}
	for name, call := range calls {
		for who, tc := range refused {
			f := newFixture()
			assert.ErrorIs(t, call(f, tc.caller), tc.want, name+"/"+who)
			assert.Empty(t, f.accounts.calls, name+"/"+who+" reached the store")
			assert.Empty(t, f.stepUp.token, name+"/"+who+" reached step-up")
		}
		require.NoError(t, call(newFixture(), adminCaller()), name)
	}
}

func TestCreateTrimsTheNameAttributesTheAdminAndNotifies(t *testing.T) {
	f := newFixture()

	account, secret, err := f.svc.Create(
		context.Background(),
		adminCaller(),
		"  Reviewer ",
	)

	require.NoError(t, err)
	assert.Equal(t, "Reviewer", f.accounts.name)
	assert.Equal(t, "human:u1", f.accounts.createdBy)
	assert.Equal(t, "secret", secret)
	assert.Equal(t, []string{"created:Reviewer"}, f.notify.events)
	assert.Equal(t, "Reviewer", account.Name)
	assert.Empty(t, f.stepUp.token, "creating needs no step-up")
}

func TestCreateWorksWithoutANotifier(t *testing.T) {
	f := newFixture()
	svc := agentaccounts.NewService(f.accounts, f.stepUp, f.families, nil)

	_, _, err := svc.Create(context.Background(), adminCaller(), "bot")

	require.NoError(t, err)
}

func TestCreateRefusesAnInvalidNameBeforeTheStore(t *testing.T) {
	for _, bad := range []string{
		"", " \t ", strings.Repeat("a", 81), "line\nbreak", "tab\there", "nul\x00",
	} {
		f := newFixture()
		_, _, err := f.svc.Create(context.Background(), adminCaller(), bad)
		assert.ErrorIs(t, err, agentaccounts.ErrInvalidName, bad)
		assert.Empty(t, f.accounts.calls, bad)
		assert.Empty(t, f.notify.events, bad)
	}
}

func TestCreateCountsCharactersNotBytes(t *testing.T) {
	for _, ok := range []string{strings.Repeat("a", 80), strings.Repeat("é", 80)} {
		_, _, err := newFixture().svc.Create(context.Background(), adminCaller(), ok)
		assert.NoError(t, err, ok)
	}
}

func TestCreateDoesNotNotifyWhenTheStoreFails(t *testing.T) {
	f := newFixture()
	f.accounts.err = errStore

	_, _, err := f.svc.Create(context.Background(), adminCaller(), "bot")

	assert.ErrorIs(t, err, errStore)
	assert.Empty(t, f.notify.events)
}

func TestRotateChecksTheStepUpWithTheSessionTokenAndCode(t *testing.T) {
	f := newFixture()

	_, secret, err := f.svc.RotateSecret(
		context.Background(), adminCaller(), uuid.NewString(), "654321",
	)

	require.NoError(t, err)
	assert.Equal(t, "new-secret", secret)
	assert.Equal(t, "session", f.stepUp.token)
	assert.Equal(t, "654321", f.stepUp.code)
	assert.Equal(t, []string{"rotate"}, f.accounts.calls)
}

func TestAFailedStepUpStopsTheChange(t *testing.T) {
	stepUpErr := agentaccounts.ErrWrongCredential
	id := uuid.NewString()

	f := newFixture()
	f.stepUp.err = stepUpErr
	_, _, err := f.svc.RotateSecret(context.Background(), adminCaller(), id, "1")
	assert.ErrorIs(t, err, stepUpErr)

	_, err = f.svc.AddToFamily(context.Background(), adminCaller(), id, "1")
	assert.ErrorIs(t, err, stepUpErr)

	assert.Empty(t, f.accounts.calls)
	assert.Empty(t, f.families.userID, "no family is created behind a failed check")
	assert.Empty(t, f.notify.events)
}

func TestStepUpNeedsAValidIDAndASignedInSession(t *testing.T) {
	f := newFixture()

	_, _, err := f.svc.RotateSecret(context.Background(), adminCaller(), "nope", "1")
	assert.ErrorIs(t, err, agentaccounts.ErrInvalidID)

	noToken := adminCaller()
	noToken.AccessToken = ""
	_, err = f.svc.AddToFamily(context.Background(), noToken, uuid.NewString(), "1")
	assert.ErrorIs(t, err, agentaccounts.ErrNotSignedIn)

	assert.Empty(t, f.stepUp.token)
	assert.Empty(t, f.accounts.calls)
}

func TestAddToFamilyUsesTheAdminsFamilyAndNotifies(t *testing.T) {
	f := newFixture()
	f.accounts.name = "family bot"

	account, err := f.svc.AddToFamily(
		context.Background(), adminCaller(), uuid.NewString(), "424242",
	)

	require.NoError(t, err)
	assert.Equal(t, "u1", f.families.userID)
	assert.Equal(t, []string{"join"}, f.accounts.calls)
	assert.Equal(t, []string{"joined:family bot"}, f.notify.events)
	assert.Equal(t, "family bot", account.Name)
}

func TestAddToFamilyStopsWhenTheFamilyCantBeResolved(t *testing.T) {
	f := newFixture()
	f.families.err = errStore

	_, err := f.svc.AddToFamily(context.Background(), adminCaller(), uuid.NewString(), "1")

	assert.ErrorIs(t, err, errStore)
	assert.Empty(t, f.accounts.calls)
	assert.Empty(t, f.notify.events)
}

func TestAddToFamilyDoesNotNotifyWhenJoiningFails(t *testing.T) {
	f := newFixture()
	f.accounts.err = errStore

	_, err := f.svc.AddToFamily(context.Background(), adminCaller(), uuid.NewString(), "1")

	assert.ErrorIs(t, err, errStore)
	assert.Empty(t, f.notify.events)
}

func TestRevokeAndRemoveRefuseABadID(t *testing.T) {
	f := newFixture()

	_, err := f.svc.Revoke(context.Background(), adminCaller(), "nope")
	assert.ErrorIs(t, err, agentaccounts.ErrInvalidID)
	_, err = f.svc.RemoveFromFamily(context.Background(), adminCaller(), "nope")
	assert.ErrorIs(t, err, agentaccounts.ErrInvalidID)

	assert.Empty(t, f.accounts.calls)
}

func TestStoreErrorsPassThrough(t *testing.T) {
	f := newFixture()
	f.accounts.err = errStore
	id := uuid.NewString()

	_, err := f.svc.List(context.Background(), adminCaller())
	assert.ErrorIs(t, err, errStore)
	_, err = f.svc.Revoke(context.Background(), adminCaller(), id)
	assert.ErrorIs(t, err, errStore)
	_, _, err = f.svc.RotateSecret(context.Background(), adminCaller(), id, "1")
	assert.ErrorIs(t, err, errStore)
	_, err = f.svc.RemoveFromFamily(context.Background(), adminCaller(), id)
	assert.ErrorIs(t, err, errStore)
}
