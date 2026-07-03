package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/internal/app"
)

// fakeRecipesStore implements recipesStore in memory for permission tests.
type fakeRecipesStore struct {
	recipe *models.Recipe
	// book access returned by GetBookAccess for any (owner, user) pair
	accessCanEdit bool
	accessOK      bool

	updated       bool
	deleted       bool
	sharedTargets []string
	updatedOwner  string
}

func (f *fakeRecipesStore) ListForUser(
	_ context.Context, _ string,
) ([]models.Recipe, error) {
	return nil, nil
}

func (f *fakeRecipesStore) GetByID(
	_ context.Context, _ uuid.UUID,
) (*models.Recipe, error) {
	cp := *f.recipe
	return &cp, nil
}

func (f *fakeRecipesStore) GetBookAccess(
	_ context.Context, _, _ string,
) (bool, bool, error) {
	return f.accessCanEdit, f.accessOK, nil
}

func (f *fakeRecipesStore) GetIngredients(
	_ context.Context, _ uuid.UUID,
) ([]models.Ingredient, error) {
	return nil, nil
}

func (f *fakeRecipesStore) Create(
	_ context.Context, recipe models.Recipe,
) (*models.Recipe, error) {
	return &recipe, nil
}

func (f *fakeRecipesStore) ReplaceIngredients(
	_ context.Context, _ uuid.UUID, _ []models.Ingredient,
) error {
	return nil
}

func (f *fakeRecipesStore) Update(_ context.Context, recipe models.Recipe) error {
	f.updated = true
	f.updatedOwner = recipe.UserID
	return nil
}

func (f *fakeRecipesStore) Delete(_ context.Context, _ uuid.UUID, _ string) error {
	f.deleted = true
	return nil
}

func (f *fakeRecipesStore) ShareBook(
	_ context.Context, _, targetUserID string, _ bool,
) error {
	f.sharedTargets = append(f.sharedTargets, targetUserID)
	return nil
}

func (f *fakeRecipesStore) UnshareBook(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeRecipesStore) ListBookShares(
	_ context.Context, _ string,
) ([]models.RecipeBookShare, error) {
	return nil, nil
}

func newRecipeFixture(owner string) *models.Recipe {
	//nolint:exhaustruct //only fields relevant to permissions
	return &models.Recipe{ID: uuid.New(), UserID: owner}
}

func httpStatus(t *testing.T, err error) int {
	t.Helper()
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	return httpErr.Status
}

func TestRecipeGet_OwnerCanEdit(t *testing.T) {
	store := &fakeRecipesStore{recipe: newRecipeFixture("owner")}
	svc := &RecipeService{repo: store}

	_, canEdit, err := svc.Get(t.Context(), uuid.New(), "owner")
	require.NoError(t, err)
	assert.True(t, canEdit)
}

func TestRecipeGet_ViewOnlyShareCannotEdit(t *testing.T) {
	store := &fakeRecipesStore{
		recipe:        newRecipeFixture("owner"),
		accessOK:      true,
		accessCanEdit: false,
	}
	svc := &RecipeService{repo: store}

	_, canEdit, err := svc.Get(t.Context(), uuid.New(), "viewer")
	require.NoError(t, err)
	assert.False(t, canEdit)
}

func TestRecipeGet_NoAccessForbidden(t *testing.T) {
	store := &fakeRecipesStore{recipe: newRecipeFixture("owner"), accessOK: false}
	svc := &RecipeService{repo: store}

	_, _, err := svc.Get(t.Context(), uuid.New(), "stranger")
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
}

func TestRecipeUpdate_EditShareKeepsOriginalOwner(t *testing.T) {
	store := &fakeRecipesStore{
		recipe:        newRecipeFixture("owner"),
		accessOK:      true,
		accessCanEdit: true,
	}
	svc := &RecipeService{repo: store}

	recipe := *store.recipe
	err := svc.Update(t.Context(), "editor", recipe)
	require.NoError(t, err)
	assert.True(t, store.updated)
	// Recipes always remain owned by their original creator.
	assert.Equal(t, "owner", store.updatedOwner)
}

func TestRecipeUpdate_ViewOnlyShareForbidden(t *testing.T) {
	store := &fakeRecipesStore{
		recipe:        newRecipeFixture("owner"),
		accessOK:      true,
		accessCanEdit: false,
	}
	svc := &RecipeService{repo: store}

	err := svc.Update(t.Context(), "viewer", *store.recipe)
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
	assert.False(t, store.updated)
}

func TestRecipeDelete_NonOwnerForbidden(t *testing.T) {
	store := &fakeRecipesStore{recipe: newRecipeFixture("owner")}
	svc := &RecipeService{repo: store}

	err := svc.Delete(t.Context(), uuid.New(), "someone-else")
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
	assert.False(t, store.deleted)
}

func TestRecipeShareBook_RejectsEmptyAndSelf(t *testing.T) {
	store := &fakeRecipesStore{}
	svc := &RecipeService{repo: store}

	err := svc.ShareBook(t.Context(), "owner", "", true)
	assert.Equal(t, http.StatusBadRequest, httpStatus(t, err))

	err = svc.ShareBook(t.Context(), "owner", "owner", true)
	assert.Equal(t, http.StatusBadRequest, httpStatus(t, err))

	assert.Empty(t, store.sharedTargets)

	require.NoError(t, svc.ShareBook(t.Context(), "owner", "friend", false))
	assert.Equal(t, []string{"friend"}, store.sharedTargets)
}
