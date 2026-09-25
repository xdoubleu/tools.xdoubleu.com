package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/database"
)

// fakeLearningPathsStore is an in-memory learningPathsStore.
type fakeLearningPathsStore struct {
	lp     *models.LearningPath
	getErr error

	getModulesErr   error
	getResourcesErr error

	createErr           error
	updateErr           error
	replaceModulesErr   error
	replaceResourcesErr error

	updated             bool
	deleted             bool
	modulesReplaced     bool
	resourcesReplaced   bool
	progressRecorded    bool
	progressCompletedTo bool

	item       *models.ItemForTask
	getItemErr error

	// resources is returned by GetResources, for resource-link tests.
	resources []models.Resource
}

func (f *fakeLearningPathsStore) ListForUser(
	_ context.Context, _ string, _, _ int32,
) ([]models.LearningPath, bool, error) {
	return nil, false, nil
}

func (f *fakeLearningPathsStore) GetByID(
	_ context.Context, _ uuid.UUID,
) (*models.LearningPath, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cp := *f.lp
	return &cp, nil
}

func (f *fakeLearningPathsStore) GetModules(
	_ context.Context, _ uuid.UUID,
) ([]models.Module, error) {
	if f.getModulesErr != nil {
		return nil, f.getModulesErr
	}
	return nil, nil
}

func (f *fakeLearningPathsStore) GetResources(
	_ context.Context, _ uuid.UUID,
) ([]models.Resource, error) {
	if f.getResourcesErr != nil {
		return nil, f.getResourcesErr
	}
	return f.resources, nil
}

func (f *fakeLearningPathsStore) Create(
	_ context.Context, lp models.LearningPath,
) (*models.LearningPath, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &lp, nil
}

func (f *fakeLearningPathsStore) Update(
	_ context.Context,
	_ models.LearningPath,
) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = true
	return nil
}

func (f *fakeLearningPathsStore) Delete(
	_ context.Context,
	_ uuid.UUID,
	_ string,
) error {
	f.deleted = true
	return nil
}

func (f *fakeLearningPathsStore) ReplaceModules(
	_ context.Context, _ uuid.UUID, _ []models.Module,
) error {
	if f.replaceModulesErr != nil {
		return f.replaceModulesErr
	}
	f.modulesReplaced = true
	return nil
}

func (f *fakeLearningPathsStore) ReplaceResources(
	_ context.Context, _ uuid.UUID, _ []models.Resource,
) error {
	if f.replaceResourcesErr != nil {
		return f.replaceResourcesErr
	}
	f.resourcesReplaced = true
	return nil
}

func (f *fakeLearningPathsStore) RecordItemProgress(
	_ context.Context, _ uuid.UUID, _ string, completed bool,
) error {
	f.progressRecorded = true
	f.progressCompletedTo = completed
	return nil
}

func (f *fakeLearningPathsStore) GetItemForUser(
	_ context.Context, _ uuid.UUID, _ string,
) (*models.ItemForTask, error) {
	if f.getItemErr != nil {
		return nil, f.getItemErr
	}
	return f.item, nil
}

func newFixture() *models.LearningPath {
	//nolint:exhaustruct //only fields relevant to ownership scoping
	return &models.LearningPath{ID: uuid.New(), UserID: "owner"}
}

// newTestService builds a service with no books/feeds lookup; these tests use
// no linked resources (resource_links_test.go covers those).
func newTestService(store learningPathsStore) *LearningPathService {
	//nolint:exhaustruct //books/feeds intentionally nil, see doc comment above
	return &LearningPathService{repo: store}
}

// fakeBookLookup is an in-memory bookLookup for error-propagation tests.
// errAfterCall, when nonzero, makes the Nth call onward return genericErr,
// so Create's second lookup can fail while its validating first one passes.
type fakeBookLookup struct {
	calls        int
	errAfterCall int
	genericErr   error
	book         *booksv1.UserBook
}

func (f *fakeBookLookup) GetLibraryBookByID(
	_ context.Context, _ string, _ uuid.UUID,
) (*booksv1.UserBook, error) {
	f.calls++
	if f.errAfterCall != 0 && f.calls >= f.errAfterCall {
		return nil, f.genericErr
	}
	return f.book, nil
}

// fakeFeedItemLookup is the feeds-side counterpart to fakeBookLookup.
type fakeFeedItemLookup struct {
	calls        int
	errAfterCall int
	genericErr   error
	item         *feeds.SharedItem
}

func (f *fakeFeedItemLookup) GetItemByID(
	_ context.Context, _ string, _ uuid.UUID,
) (*feeds.SharedItem, error) {
	f.calls++
	if f.errAfterCall != 0 && f.calls >= f.errAfterCall {
		return nil, f.genericErr
	}
	return f.item, nil
}

func TestGet_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	lp, err := svc.Get(t.Context(), uuid.New(), "owner")
	require.NoError(t, err)
	assert.Equal(t, "owner", lp.UserID)
}

func TestGet_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	_, err := svc.Get(t.Context(), uuid.New(), "other")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestGet_PropagatesModulesError(t *testing.T) {
	modulesErr := errors.New("modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getModulesErr: modulesErr}
	svc := newTestService(store)

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, modulesErr)
}

func TestGet_PropagatesResourcesError(t *testing.T) {
	resourcesErr := errors.New("resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getResourcesErr: resourcesErr}
	svc := newTestService(store)

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, resourcesErr)
}

func TestUpdate_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "other", *store.lp)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.False(t, store.updated)
}

func TestUpdate_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "owner", *store.lp)
	require.NoError(t, err)
	assert.True(t, store.updated)
	assert.True(t, store.modulesReplaced)
	assert.True(t, store.resourcesReplaced)
}

func TestUpdate_PropagatesGetByIDError(t *testing.T) {
	getErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getErr: getErr}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "owner", *newFixture())
	assert.ErrorIs(t, err, getErr)
}

func TestUpdate_PropagatesUpdateError(t *testing.T) {
	updateErr := errors.New("update db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), updateErr: updateErr}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, updateErr)
}

func TestUpdate_PropagatesReplaceModulesError(t *testing.T) {
	replaceErr := errors.New("replace modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), replaceModulesErr: replaceErr}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, replaceErr)
}

func TestUpdate_PropagatesReplaceResourcesError(t *testing.T) {
	replaceErr := errors.New("replace resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), replaceResourcesErr: replaceErr}
	svc := newTestService(store)

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, replaceErr)
}

func TestDelete_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	err := svc.Delete(t.Context(), uuid.New(), "other")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.False(t, store.deleted)
}

func TestDelete_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	err := svc.Delete(t.Context(), uuid.New(), "owner")
	require.NoError(t, err)
	assert.True(t, store.deleted)
}

func TestCreate_ScopesToUser(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	svc := newTestService(store)

	//nolint:exhaustruct //other fields optional
	created, err := svc.Create(t.Context(), "member", models.LearningPath{
		Title: "New Path",
	})
	require.NoError(t, err)
	assert.Equal(t, "member", created.UserID)
	assert.True(t, store.modulesReplaced)
	assert.True(t, store.resourcesReplaced)
}

func TestCreate_PropagatesCreateError(t *testing.T) {
	createErr := errors.New("create db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{createErr: createErr}
	svc := newTestService(store)

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, createErr)
}

func TestCreate_PropagatesReplaceModulesError(t *testing.T) {
	replaceErr := errors.New("replace modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{replaceModulesErr: replaceErr}
	svc := newTestService(store)

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, replaceErr)
}

func TestCreate_PropagatesReplaceResourcesError(t *testing.T) {
	replaceErr := errors.New("replace resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{replaceResourcesErr: replaceErr}
	svc := newTestService(store)

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, replaceErr)
}

func TestGetByID_PropagatesError(t *testing.T) {
	getErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getErr: getErr}
	svc := newTestService(store)

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, getErr)
}

func TestRecordItemProgress_DelegatesToRepo(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	svc := newTestService(store)

	err := svc.RecordItemProgress(t.Context(), "owner", uuid.New(), true)
	require.NoError(t, err)
	assert.True(t, store.progressRecorded)
	assert.True(t, store.progressCompletedTo)
}

// --- resource-link error propagation -------------------------------------
//
// An infrastructure failure from the books/feeds lookup must propagate as-is,
// not be swallowed or turned into a 400.

func TestGet_ResolveResourceLinks_PropagatesBookLookupError(t *testing.T) {
	lookupErr := errors.New("books lookup infra error")
	bookID := uuid.New()
	lp := newFixture()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{
		lp:        lp,
		resources: []models.Resource{{LinkedBookID: &bookID}},
	}
	//nolint:exhaustruct //feeds intentionally nil, unused on this path
	svc := &LearningPathService{
		repo:  store,
		books: &fakeBookLookup{genericErr: lookupErr, errAfterCall: 1},
	}

	_, err := svc.Get(t.Context(), lp.ID, lp.UserID)
	assert.ErrorIs(t, err, lookupErr)
}

func TestGet_ResolveResourceLinks_PropagatesFeedItemLookupError(t *testing.T) {
	lookupErr := errors.New("feeds lookup infra error")
	itemID := uuid.New()
	lp := newFixture()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{
		lp:        lp,
		resources: []models.Resource{{LinkedFeedItemID: &itemID}},
	}
	//nolint:exhaustruct //books intentionally nil, unused on this path
	svc := &LearningPathService{
		repo:  store,
		feeds: &fakeFeedItemLookup{genericErr: lookupErr, errAfterCall: 1},
	}

	_, err := svc.Get(t.Context(), lp.ID, lp.UserID)
	assert.ErrorIs(t, err, lookupErr)
}

func TestCreate_ValidateResourceLinks_PropagatesBookLookupError(t *testing.T) {
	lookupErr := errors.New("books lookup infra error")
	bookID := uuid.New()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	//nolint:exhaustruct //feeds intentionally nil, unused on this path
	svc := &LearningPathService{
		repo:  store,
		books: &fakeBookLookup{genericErr: lookupErr, errAfterCall: 1},
	}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{
		Title:     "New Path",
		Resources: []models.Resource{{LinkedBookID: &bookID}},
	})
	assert.ErrorIs(t, err, lookupErr)
	assert.False(t, store.resourcesReplaced)
}

func TestCreate_ValidateResourceLinks_PropagatesFeedItemLookupError(t *testing.T) {
	lookupErr := errors.New("feeds lookup infra error")
	itemID := uuid.New()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	//nolint:exhaustruct //books intentionally nil, unused on this path
	svc := &LearningPathService{
		repo:  store,
		feeds: &fakeFeedItemLookup{genericErr: lookupErr, errAfterCall: 1},
	}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{
		Title:     "New Path",
		Resources: []models.Resource{{LinkedFeedItemID: &itemID}},
	})
	assert.ErrorIs(t, err, lookupErr)
	assert.False(t, store.resourcesReplaced)
}

// TestCreate_ResolveResourceLinks_PropagatesBookLookupError covers Create's
// post-persist resolveResourceLinks lookup.
func TestCreate_ResolveResourceLinks_PropagatesBookLookupError(t *testing.T) {
	lookupErr := errors.New("books lookup infra error")
	bookID := uuid.New()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	//nolint:exhaustruct //feeds intentionally nil, unused on this path
	svc := &LearningPathService{
		repo: store,
		// Lets validateResourceLinks pass; only resolveResourceLinks errors.
		books: &fakeBookLookup{genericErr: lookupErr, errAfterCall: 2},
	}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{
		Title:     "New Path",
		Resources: []models.Resource{{LinkedBookID: &bookID}},
	})
	assert.ErrorIs(t, err, lookupErr)
	assert.True(t, store.resourcesReplaced)
}
