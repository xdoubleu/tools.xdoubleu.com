package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/internal/app"
)

// fakeRecipesStore implements recipesStore in memory for family-scoping tests.
type fakeRecipesStore struct {
	recipe      *models.Recipe
	ingredients []models.Ingredient
	getErr      error

	updated      bool
	deleted      bool
	updatedOwner string
}

func (f *fakeRecipesStore) ListForFamily(
	_ context.Context, _ uuid.UUID, _, _ int32,
) ([]models.Recipe, bool, error) {
	return nil, false, nil
}

func (f *fakeRecipesStore) GetByID(
	_ context.Context, _ uuid.UUID,
) (*models.Recipe, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cp := *f.recipe
	return &cp, nil
}

func (f *fakeRecipesStore) GetIngredients(
	_ context.Context, _ uuid.UUID,
) ([]models.Ingredient, error) {
	return f.ingredients, nil
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

func (f *fakeRecipesStore) Delete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	f.deleted = true
	return nil
}

// fakeFamilyStore maps a user to a fixed family ID for tests.
type fakeFamilyStore struct {
	families map[string]uuid.UUID
	err      error
}

func (f *fakeFamilyStore) EnsureFamily(
	_ context.Context, userID string,
) (uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, f.err
	}
	return f.families[userID], nil
}

//nolint:gochecknoglobals //shared fixtures for family-scoping tests
var (
	familyA = uuid.New()
	familyB = uuid.New()
)

func newRecipeFixture() *models.Recipe {
	//nolint:exhaustruct //only fields relevant to family scoping
	return &models.Recipe{ID: uuid.New(), UserID: "owner", FamilyID: familyA}
}

func newFamilyStore() *fakeFamilyStore {
	//nolint:exhaustruct //err intentionally zero
	return &fakeFamilyStore{
		families: map[string]uuid.UUID{
			"owner":  familyA,
			"member": familyA,
			"editor": familyA,
			"viewer": familyA,
			"other":  familyB,
		},
	}
}

func httpStatus(t *testing.T, err error) int {
	t.Helper()
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	return httpErr.Status
}

func TestRecipeGet_FamilyMemberCanEdit(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, canEdit, err := svc.Get(t.Context(), uuid.New(), "member")
	require.NoError(t, err)
	assert.True(t, canEdit)
}

func TestRecipeGet_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, _, err := svc.Get(t.Context(), uuid.New(), "other")
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
}

func TestRecipeUpdate_FamilyMemberKeepsOriginalOwner(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	recipe := *store.recipe
	err := svc.Update(t.Context(), "editor", recipe)
	require.NoError(t, err)
	assert.True(t, store.updated)
	// Recipes always remain attributed to their original creator.
	assert.Equal(t, "owner", store.updatedOwner)
}

func TestRecipeUpdate_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	err := svc.Update(t.Context(), "other", *store.recipe)
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
	assert.False(t, store.updated)
}

func TestRecipeDelete_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	err := svc.Delete(t.Context(), uuid.New(), "other")
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
	assert.False(t, store.deleted)
}

func TestRecipeDelete_FamilyMemberAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	err := svc.Delete(t.Context(), uuid.New(), "member")
	require.NoError(t, err)
	assert.True(t, store.deleted)
}

func TestRecipeGetScaled_DoublesIngredientAmounts(t *testing.T) {
	recipe := newRecipeFixture()
	recipe.BaseServings = 2
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{
		recipe: recipe,
		ingredients: []models.Ingredient{
			{Name: "Flour", Amount: 100, Unit: "g"},
		},
	}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, _, servings, scaled, err := svc.GetScaled(t.Context(), uuid.New(), "owner", 4)
	require.NoError(t, err)
	assert.Equal(t, 4, servings)
	assert.Equal(t, float64(200), scaled[0].Amount)
}

func TestRecipeGetScaled_ZeroRequestKeepsBaseServings(t *testing.T) {
	recipe := newRecipeFixture()
	recipe.BaseServings = 2
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{
		recipe: recipe,
		ingredients: []models.Ingredient{
			{Name: "Flour", Amount: 100, Unit: "g"},
		},
	}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, _, servings, scaled, err := svc.GetScaled(t.Context(), uuid.New(), "owner", 0)
	require.NoError(t, err)
	assert.Equal(t, 2, servings)
	assert.Equal(t, float64(100), scaled[0].Amount)
}

func TestRecipeGetScaled_PropagatesGetError(t *testing.T) {
	getErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture(), getErr: getErr}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, _, _, _, err := svc.GetScaled(t.Context(), uuid.New(), "owner", 4)
	assert.ErrorIs(t, err, getErr)
}

func TestRecipeCreate_ScopesToUsersFamily(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct //other fields optional
	created, err := svc.Create(t.Context(), "member", models.Recipe{
		Name: "Pasta",
	})
	require.NoError(t, err)
	assert.Equal(t, "member", created.UserID)
	assert.Equal(t, familyA, created.FamilyID)
}

func TestRecipeList_ScopesToUsersFamily(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	_, _, err := svc.List(t.Context(), "member", 10, 0)
	require.NoError(t, err)
}

// Every RecipeService method resolves the caller's family before touching
// the repo; a family-resolution failure must propagate.
func TestFamilyResolutionErrors_Propagate(t *testing.T) {
	familyErr := errors.New("family error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	family := &fakeFamilyStore{err: familyErr}
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: family}
	ctx := t.Context()

	_, _, err := svc.List(ctx, "member", 10, 0)
	assert.ErrorIs(t, err, familyErr)

	_, _, err = svc.Get(ctx, uuid.New(), "member")
	assert.ErrorIs(t, err, familyErr)

	//nolint:exhaustruct //other fields optional
	_, err = svc.Create(ctx, "member", models.Recipe{Name: "Pasta"})
	assert.ErrorIs(t, err, familyErr)

	err = svc.Update(ctx, "member", *store.recipe)
	assert.ErrorIs(t, err, familyErr)

	err = svc.Delete(ctx, uuid.New(), "member")
	assert.ErrorIs(t, err, familyErr)
}
