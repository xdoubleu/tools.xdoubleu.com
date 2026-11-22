//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type fakeResync struct {
	started   []bool
	cancelled int
}

func (f *fakeResync) Start(force bool) { f.started = append(f.started, force) }
func (f *fakeResync) Cancel()          { f.cancelled++ }

func user(role string) *sharedmodels.User {
	//nolint:exhaustruct // only fields under test
	return &sharedmodels.User{ID: "u1", Role: sharedmodels.Role(role)}
}

func TestRequireAdmin(t *testing.T) {
	require.ErrorIs(t, RequireAdmin(nil), ErrUnauthenticated)
	require.ErrorIs(
		t, RequireAdmin(user(string(sharedmodels.RoleUser))), ErrAdminRequired,
	)
	require.NoError(t, RequireAdmin(user(string(sharedmodels.RoleAdmin))))
}

func TestNormalizeISBN13(t *testing.T) {
	got, err := NormalizeISBN13("978-0 14-044911-2")
	require.NoError(t, err)
	assert.Equal(t, "9780140449112", got)

	_, err = NormalizeISBN13("12345")
	require.ErrorIs(t, err, ErrISBNLength)

	_, err = NormalizeISBN13("97801404491x2")
	require.ErrorIs(t, err, ErrISBNDigits)
}

func TestCatalog_RejectsNonAdminBeforeTouchingStorage(t *testing.T) {
	resync := &fakeResync{}
	// nil book and progress services: any admin-gated call reaching them panics.
	svc := NewCatalogService(nil, nil, nil, resync)
	ctx := context.Background()
	member := user(string(sharedmodels.RoleUser))
	id := uuid.New()

	_, err := svc.FindDuplicates(ctx, member)
	require.ErrorIs(t, err, ErrAdminRequired)
	_, err = svc.MergeBooks(ctx, member, MergeRequest{Winner: id})
	require.ErrorIs(t, err, ErrAdminRequired)
	require.ErrorIs(t, svc.StartResync(member, true), ErrAdminRequired)
	require.ErrorIs(t, svc.CancelResync(member), ErrAdminRequired)
	_, err = svc.ListResyncProposals(ctx, member)
	require.ErrorIs(t, err, ErrAdminRequired)
	require.ErrorIs(t, svc.ApplyResyncChoice(ctx, member, id, "unicat"), ErrAdminRequired)
	_, err = svc.GetBookSources(ctx, member, id, "", "")
	require.ErrorIs(t, err, ErrAdminRequired)
	require.ErrorIs(
		t, svc.ApplyBookSource(ctx, member, id, "unicat", 0, "", ""), ErrAdminRequired,
	)
	require.ErrorIs(t, svc.SetBookISBN(ctx, member, id, "9780140449112"), ErrAdminRequired)
	_, err = svc.UpdateBook(ctx, member, id, &models.Book{}, "")
	require.ErrorIs(t, err, ErrAdminRequired)
	_, err = svc.GetSourceStats(ctx, member)
	require.ErrorIs(t, err, ErrAdminRequired)
	_, err = svc.ListBooksInExactSources(ctx, member, nil)
	require.ErrorIs(t, err, ErrAdminRequired)

	_, err = svc.FindDuplicates(ctx, nil)
	require.ErrorIs(t, err, ErrUnauthenticated)
	assert.Empty(t, resync.started)
	assert.Zero(t, resync.cancelled)
}

func TestCatalog_ResyncControl(t *testing.T) {
	resync := &fakeResync{}
	svc := NewCatalogService(nil, nil, nil, resync)
	admin := user(string(sharedmodels.RoleAdmin))

	require.NoError(t, svc.StartResync(admin, true))
	require.NoError(t, svc.StartResync(admin, false))
	require.NoError(t, svc.CancelResync(admin))

	assert.Equal(t, []bool{true, false}, resync.started)
	assert.Equal(t, 1, resync.cancelled)
}

func TestCatalog_AdminEditValidation(t *testing.T) {
	svc := NewCatalogService(nil, nil, nil, &fakeResync{})
	ctx := context.Background()
	admin := user(string(sharedmodels.RoleAdmin))
	id := uuid.New()

	require.ErrorIs(t, svc.SetBookISBN(ctx, admin, id, "123"), ErrISBNLength)

	_, err := svc.UpdateBook(ctx, admin, id, nil, "")
	require.ErrorIs(t, err, ErrMetadataRequired)

	bad := "123"
	_, err = svc.UpdateBook(ctx, admin, id, &models.Book{ISBN13: &bad}, "")
	require.ErrorIs(t, err, ErrISBNLength)
}
