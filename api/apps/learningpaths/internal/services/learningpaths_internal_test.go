package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// fakeLearningPathsStore implements learningPathsStore in memory for
// ownership-scoping and error-propagation tests.
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
	return nil, nil
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

func newFixture() *models.LearningPath {
	//nolint:exhaustruct //only fields relevant to ownership scoping
	return &models.LearningPath{ID: uuid.New(), UserID: "owner"}
}

func TestGet_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

	lp, err := svc.Get(t.Context(), uuid.New(), "owner")
	require.NoError(t, err)
	assert.Equal(t, "owner", lp.UserID)
}

func TestGet_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

	_, err := svc.Get(t.Context(), uuid.New(), "other")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestGet_PropagatesModulesError(t *testing.T) {
	modulesErr := errors.New("modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getModulesErr: modulesErr}
	svc := &LearningPathService{repo: store}

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, modulesErr)
}

func TestGet_PropagatesResourcesError(t *testing.T) {
	resourcesErr := errors.New("resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getResourcesErr: resourcesErr}
	svc := &LearningPathService{repo: store}

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, resourcesErr)
}

func TestUpdate_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

	err := svc.Update(t.Context(), "other", *store.lp)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.False(t, store.updated)
}

func TestUpdate_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

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
	svc := &LearningPathService{repo: store}

	err := svc.Update(t.Context(), "owner", *newFixture())
	assert.ErrorIs(t, err, getErr)
}

func TestUpdate_PropagatesUpdateError(t *testing.T) {
	updateErr := errors.New("update db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), updateErr: updateErr}
	svc := &LearningPathService{repo: store}

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, updateErr)
}

func TestUpdate_PropagatesReplaceModulesError(t *testing.T) {
	replaceErr := errors.New("replace modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), replaceModulesErr: replaceErr}
	svc := &LearningPathService{repo: store}

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, replaceErr)
}

func TestUpdate_PropagatesReplaceResourcesError(t *testing.T) {
	replaceErr := errors.New("replace resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), replaceResourcesErr: replaceErr}
	svc := &LearningPathService{repo: store}

	err := svc.Update(t.Context(), "owner", *store.lp)
	assert.ErrorIs(t, err, replaceErr)
}

func TestDelete_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

	err := svc.Delete(t.Context(), uuid.New(), "other")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.False(t, store.deleted)
}

func TestDelete_OwnerAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := &LearningPathService{repo: store}

	err := svc.Delete(t.Context(), uuid.New(), "owner")
	require.NoError(t, err)
	assert.True(t, store.deleted)
}

func TestCreate_ScopesToUser(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	svc := &LearningPathService{repo: store}

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
	svc := &LearningPathService{repo: store}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, createErr)
}

func TestCreate_PropagatesReplaceModulesError(t *testing.T) {
	replaceErr := errors.New("replace modules db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{replaceModulesErr: replaceErr}
	svc := &LearningPathService{repo: store}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, replaceErr)
}

func TestCreate_PropagatesReplaceResourcesError(t *testing.T) {
	replaceErr := errors.New("replace resources db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{replaceResourcesErr: replaceErr}
	svc := &LearningPathService{repo: store}

	//nolint:exhaustruct //other fields optional
	_, err := svc.Create(t.Context(), "member", models.LearningPath{Title: "New Path"})
	assert.ErrorIs(t, err, replaceErr)
}

func TestGetByID_PropagatesError(t *testing.T) {
	getErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture(), getErr: getErr}
	svc := &LearningPathService{repo: store}

	_, err := svc.Get(t.Context(), uuid.New(), "owner")
	assert.ErrorIs(t, err, getErr)
}

func TestRecordItemProgress_DelegatesToRepo(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	svc := &LearningPathService{repo: store}

	err := svc.RecordItemProgress(t.Context(), "owner", uuid.New(), true)
	require.NoError(t, err)
	assert.True(t, store.progressRecorded)
	assert.True(t, store.progressCompletedTo)
}
