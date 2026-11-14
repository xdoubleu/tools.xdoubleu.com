//nolint:exhaustruct // test doubles set only what each case needs
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

var errProvider = errors.New("provider down")

// fakeDirectory stands in for Zitadel and tracks which users exist there.
type fakeDirectory struct {
	users          map[string]bool
	deactivated    map[string]bool
	createErr      error
	inviteErr      error
	deactivateErr  error
	invited        []string
	nextID         string
	createdForName string
}

func newFakeDirectory() *fakeDirectory {
	return &fakeDirectory{
		users: map[string]bool{}, deactivated: map[string]bool{}, nextID: "zid-1",
	}
}

func (d *fakeDirectory) CreateHuman(_ context.Context, _, name string) (string, error) {
	if d.createErr != nil {
		return "", d.createErr
	}
	d.createdForName = name
	d.users[d.nextID] = true
	return d.nextID, nil
}

func (d *fakeDirectory) SendInvite(_ context.Context, id string) error {
	if d.inviteErr != nil {
		return d.inviteErr
	}
	d.invited = append(d.invited, id)
	return nil
}

func (d *fakeDirectory) Deactivate(_ context.Context, id string) error {
	if d.deactivateErr != nil {
		return d.deactivateErr
	}
	d.deactivated[id] = true
	return nil
}

func (d *fakeDirectory) Delete(_ context.Context, id string) error {
	delete(d.users, id)
	return nil
}

// memStore is the users table in memory.
type memStore struct {
	usersStore
	users       map[string]*User
	createErr   error
	deleteErr   error
	deactivated map[string]bool
	revoked     []string
}

func newMemStore() *memStore {
	return &memStore{users: map[string]*User{}, deactivated: map[string]bool{}}
}

func (s *memStore) GetUserByEmail(_ context.Context, email string) (*User, error) {
	for _, u := range s.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errNotFound
}

func (s *memStore) GetUserByID(_ context.Context, id string) (*User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, errNotFound
}

func (s *memStore) CreateOIDCUser(
	_ context.Context,
	email, subject string,
) (*User, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	u := &User{ID: "user-" + subject, Email: email, OIDCSubject: subject}
	s.users[u.ID] = u
	return u, nil
}

func (s *memStore) DeleteUser(_ context.Context, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.users, id)
	return nil
}

func (s *memStore) SetDeactivated(_ context.Context, id string) error {
	s.deactivated[id] = true
	s.users[id].Deactivated = true
	return nil
}

func (s *memStore) DeleteAllRefreshTokensForUser(_ context.Context, id string) error {
	s.revoked = append(s.revoked, id)
	return nil
}

func (s *memStore) RevokeOAuthGrants(context.Context, string) error { return nil }

// memAppUsers is global.app_users in memory.
type memAppUsers struct {
	rows       map[string]string
	names      map[string]string
	upsertErr  error
	nameErr    error
	deleteCall int
}

func newMemAppUsers() *memAppUsers {
	return &memAppUsers{rows: map[string]string{}, names: map[string]string{}}
}

func (a *memAppUsers) Upsert(_ context.Context, id, email string) error {
	if a.upsertErr != nil {
		return a.upsertErr
	}
	a.rows[id] = email
	return nil
}

func (a *memAppUsers) SetDisplayName(_ context.Context, id, name string) error {
	if a.nameErr != nil {
		return a.nameErr
	}
	a.names[id] = name
	return nil
}

func (a *memAppUsers) Delete(_ context.Context, id string) error {
	a.deleteCall++
	delete(a.rows, id)
	delete(a.names, id)
	return nil
}

func (a *memAppUsers) GetByID(context.Context, string) (*models.User, error) {
	return nil, errors.New("unused")
}

func provisioningService(
	dir *fakeDirectory, store *memStore, app *memAppUsers,
) *LocalService {
	svc := &LocalService{
		usersStore: store, appUsersRepo: app,
		userCache: newUserCache(0),
	}
	if dir != nil {
		svc.Directory = dir
	}
	return svc
}

func TestCreateUserLinksAndInvites(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	svc := provisioningService(dir, store, app)

	user, err := svc.CreateUser(context.Background(), "  Ada@Example.com ", " Ada L ")
	require.NoError(t, err)

	assert.Equal(t, "ada@example.com", user.Email)
	assert.Equal(t, "zid-1", user.OIDCSubject)
	assert.Equal(t, []string{"zid-1"}, dir.invited)
	assert.Equal(t, "Ada L", dir.createdForName)
	assert.Equal(t, "ada@example.com", app.rows[user.ID])
	assert.Equal(t, "Ada L", app.names[user.ID])
}

func TestCreateUserRejectsBeforeTouchingTheProvider(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	svc := provisioningService(dir, store, app)
	ctx := context.Background()

	_, err := svc.CreateUser(ctx, "not an email", "")
	require.ErrorIs(t, err, ErrInvalidEmail)
	_, err = svc.CreateUser(ctx, "Ada <ada@example.com>", "")
	require.ErrorIs(t, err, ErrInvalidEmail)

	_, err = svc.CreateUser(ctx, "ada@example.com", "")
	require.NoError(t, err)
	dir.nextID = "zid-2"
	_, err = svc.CreateUser(ctx, "ada@example.com", "")
	require.ErrorIs(t, err, ErrUserExists)
	assert.False(t, dir.users["zid-2"])

	_, err = provisioningService(nil, store, app).CreateUser(ctx, "b@example.com", "")
	require.ErrorIs(t, err, ErrDirectoryOff)
}

func TestCreateUserProviderFailureCreatesNothing(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	dir.createErr = errProvider

	_, err := provisioningService(dir, store, app).
		CreateUser(context.Background(), "ada@example.com", "")

	require.ErrorIs(t, err, errProvider)
	assert.Empty(t, store.users)
}

// The halfway failures: each leaves neither a provider user nor a local row.
func TestCreateUserFailureHalfwayLeavesNoOrphan(t *testing.T) {
	cases := map[string]func(*fakeDirectory, *memStore, *memAppUsers) error{
		"insert fails": func(_ *fakeDirectory, s *memStore, _ *memAppUsers) error {
			s.createErr = errStore
			return errStore
		},
		"app user fails": func(_ *fakeDirectory, _ *memStore, a *memAppUsers) error {
			a.upsertErr = errStore
			return errStore
		},
		"display name fails": func(_ *fakeDirectory, _ *memStore, a *memAppUsers) error {
			a.nameErr = errStore
			return errStore
		},
		"invite fails": func(d *fakeDirectory, _ *memStore, _ *memAppUsers) error {
			d.inviteErr = errProvider
			return errProvider
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
			want := setup(dir, store, app)

			_, err := provisioningService(dir, store, app).
				CreateUser(context.Background(), "ada@example.com", "Ada")

			require.ErrorIs(t, err, want)
			assert.Empty(t, dir.users, "provider user removed")
			assert.Empty(t, store.users, "local user removed")
			assert.Empty(t, app.rows, "app user removed")
		})
	}
}

func TestCreateUserUndoSurvivesCleanupFailure(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	dir.inviteErr = errProvider
	store.deleteErr = errStore

	_, err := provisioningService(dir, store, app).
		CreateUser(context.Background(), "ada@example.com", "")

	require.ErrorIs(t, err, errProvider)
	assert.Empty(t, dir.users)
}

func TestDeactivateUserBlocksBothSidesAndSignsOut(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	svc := provisioningService(dir, store, app)
	user, err := svc.CreateUser(context.Background(), "ada@example.com", "")
	require.NoError(t, err)

	require.NoError(t, svc.DeactivateUser(context.Background(), user.ID))

	assert.True(t, dir.deactivated["zid-1"])
	assert.True(t, store.deactivated[user.ID])
	assert.Equal(t, []string{user.ID}, store.revoked)
}

func TestDeactivateUserProviderFailureLeavesUserActive(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	svc := provisioningService(dir, store, app)
	user, err := svc.CreateUser(context.Background(), "ada@example.com", "")
	require.NoError(t, err)
	dir.deactivateErr = errProvider

	err = svc.DeactivateUser(context.Background(), user.ID)

	require.ErrorIs(t, err, errProvider)
	assert.False(t, store.deactivated[user.ID])
}

func TestDeactivateUserEdgeCases(t *testing.T) {
	dir, store, app := newFakeDirectory(), newMemStore(), newMemAppUsers()
	ctx := context.Background()

	require.ErrorIs(t,
		provisioningService(dir, store, app).DeactivateUser(ctx, "nope"),
		ErrUserNotFound)

	// A user never linked to the provider is only blocked here.
	store.users["local"] = &User{ID: "local", Email: "l@example.com"}
	require.NoError(t, provisioningService(nil, store, app).DeactivateUser(ctx, "local"))
	assert.True(t, store.deactivated["local"])

	store.users["linked"] = &User{ID: "linked", OIDCSubject: "zid-9"}
	require.ErrorIs(t,
		provisioningService(nil, store, app).DeactivateUser(ctx, "linked"),
		ErrDirectoryOff)
}

func TestDeactivatedUserCannotSignIn(t *testing.T) {
	store := newMemStore()
	store.users["u"] = &User{
		ID: "u", OIDCSubject: "sub", Deactivated: true,
	}
	svc := provisioningService(nil, store, nil)
	svc.jwtSecret = []byte("secret")
	svc.accessExpiry = "1h"

	token, err := svc.mintAccessToken("u", aal2)
	require.NoError(t, err)
	_, err = svc.GetUser(context.Background(), token)
	require.Error(t, err)
}

func (a *memAppUsers) GetAll(context.Context) ([]models.User, error) { return nil, nil }

func (s *memStore) GetVerifiedTOTPFactor(context.Context, string) (*TOTPFactor, error) {
	return nil, errNotFound
}
