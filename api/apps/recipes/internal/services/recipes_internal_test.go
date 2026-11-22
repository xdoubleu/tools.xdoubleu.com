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
	"tools.xdoubleu.com/internal/kit/database"
)

// fakeRecipesStore implements recipesStore in memory for family-scoping tests.
type fakeRecipesStore struct {
	recipe      *models.Recipe
	ingredients []models.Ingredient
	getErr      error

	updated      bool
	deleted      bool
	updatedOwner string
	updatedDraft bool
}

func (f *fakeRecipesStore) ListForFamily(
	_ context.Context, _ uuid.UUID, _, _ string, _, _ int32,
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
	f.updatedDraft = recipe.IsDraft
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
	err := svc.update(t.Context(), "editor", recipe, false)
	require.NoError(t, err)
	assert.True(t, store.updated)
	assert.Equal(t, "owner", store.updatedOwner)
}

func TestRecipeUpdate_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	err := svc.update(t.Context(), "other", *store.recipe, false)
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
	created, err := svc.create(t.Context(), "member", models.Recipe{
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

	_, _, err := svc.List(t.Context(), "member", "", "", 10, 0)
	require.NoError(t, err)
}

// A family-resolution failure must propagate from every method.
func TestFamilyResolutionErrors_Propagate(t *testing.T) {
	familyErr := errors.New("family error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	family := &fakeFamilyStore{err: familyErr}
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: family}
	ctx := t.Context()

	_, _, err := svc.List(ctx, "member", "", "", 10, 0)
	assert.ErrorIs(t, err, familyErr)

	_, _, err = svc.Get(ctx, uuid.New(), "member")
	assert.ErrorIs(t, err, familyErr)

	//nolint:exhaustruct //other fields optional
	_, err = svc.create(ctx, "member", models.Recipe{Name: "Pasta"})
	assert.ErrorIs(t, err, familyErr)

	err = svc.update(ctx, "member", *store.recipe, false)
	assert.ErrorIs(t, err, familyErr)

	err = svc.Delete(ctx, uuid.New(), "member")
	assert.ErrorIs(t, err, familyErr)
}

func TestRecipeCreate_ReplayReturnsStoredRecipe(t *testing.T) {
	fixture := newRecipeFixture()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{
		recipe:      fixture,
		ingredients: []models.Ingredient{{Name: "kept"}}, //nolint:exhaustruct // name only
	}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct //only the replayed ID matters
	got, err := svc.create(
		t.Context(),
		"member",
		models.Recipe{ID: fixture.ID, Name: "new"},
	)
	require.NoError(t, err)
	assert.Equal(t, fixture.ID, got.ID)
	assert.Equal(t, "kept", got.Ingredients[0].Name)
}

func TestRecipeCreate_ReplayOfAnotherFamilysIDIsNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct //only the ID matters
	_, err := svc.create(t.Context(), "other", models.Recipe{ID: store.recipe.ID})
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestRecipeCreate_NewClientIDIsInserted(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{getErr: database.ErrResourceNotFound}
	svc := &RecipeService{repo: store, family: newFamilyStore()}
	id := uuid.New()

	//nolint:exhaustruct //only the ID matters
	got, err := svc.create(t.Context(), "owner", models.Recipe{ID: id})
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
}

func TestRecipeCreate_LookupErrorIsReturned(t *testing.T) {
	lookupErr := errors.New("db down")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{getErr: lookupErr}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct //only the ID matters
	_, err := svc.create(t.Context(), "owner", models.Recipe{ID: uuid.New()})
	assert.ErrorIs(t, err, lookupErr)
}

func TestCategories_RejectUnknownValues(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}
	ctx := t.Context()

	_, _, err := svc.List(ctx, "member", "dessert", "", 10, 0)
	assert.Error(t, err)
	_, _, err = svc.List(ctx, "member", "", "medium", 10, 0)
	assert.Error(t, err)

	//nolint:exhaustruct //only categories matter
	_, err = svc.create(ctx, "member", models.Recipe{MealType: "dessert"})
	assert.Error(t, err)
	//nolint:exhaustruct //only categories matter
	err = svc.update(ctx, "member", models.Recipe{Effort: "medium"}, false)
	assert.Error(t, err)

	_, _, err = svc.List(ctx, "member", models.MealTypeLight, models.EffortLow, 10, 0)
	assert.NoError(t, err)
}

func TestRecipeInput_Normalises(t *testing.T) {
	group := "  sauce "
	//nolint:exhaustruct // only the fields under test
	got := RecipeInput{
		Name:         "Bowl",
		Steps:        []string{" 1. a ", "  ", "2. b"},
		BaseServings: 0,
		Ingredients: []IngredientInput{
			{
				Name: "rijst", Amount: 200, Unit: " g ",
				GroupName: group, AlternativeGroup: " base ",
			},
			{Name: "", Amount: 1},
			{Name: "ui", Amount: 1, AlternativeGroup: "base"},
		},
	}.toRecipe()

	assert.Equal(t, "1. a\n2. b", got.Instructions)
	assert.Equal(t, 2, got.BaseServings)
	require.Len(t, got.Ingredients, 2)
	assert.Equal(t, "g", got.Ingredients[0].Unit)
	assert.Equal(t, 0, got.Ingredients[0].SortOrder)
	require.NotNil(t, got.Ingredients[0].GroupName)
	assert.Equal(t, "sauce", *got.Ingredients[0].GroupName)
	require.NotNil(t, got.Ingredients[0].AlternativeGroup)
	assert.Equal(t, "base", *got.Ingredients[0].AlternativeGroup)
	assert.Equal(t, 2, got.Ingredients[1].SortOrder, "blank lines keep their position")
	assert.Nil(t, got.Ingredients[1].GroupName)
	require.NotNil(t, got.Ingredients[1].AlternativeGroup)
}

func TestRecipeInput_KeepsGivenServings(t *testing.T) {
	//nolint:exhaustruct // only the fields under test
	assert.Equal(t, 4, RecipeInput{BaseServings: 4}.toRecipe().BaseServings)
}

func TestRecipeCreate_AsAgentDraftForcesDraft(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct // only the fields under test
	published, err := svc.Create(t.Context(), "member", RecipeInput{Name: "A"})
	require.NoError(t, err)
	assert.False(t, published.IsDraft)

	//nolint:exhaustruct // only the fields under test
	draft, err := svc.Create(t.Context(), "member", RecipeInput{Name: "B"}, AsAgentDraft())
	require.NoError(t, err)
	assert.True(t, draft.IsDraft)
}

func TestRecipeUpdateKeepingDraft_KeepsStoredFlag(t *testing.T) {
	for _, stored := range []bool{true, false} {
		fixture := newRecipeFixture()
		fixture.IsDraft = stored
		fixture.BaseServings = 2
		//nolint:exhaustruct //unset fields are the fixture defaults
		store := &fakeRecipesStore{recipe: fixture}
		svc := &RecipeService{repo: store, family: newFamilyStore()}

		//nolint:exhaustruct // only the fields under test
		got, err := svc.UpdateKeepingDraft(
			t.Context(), "member",
			RecipeInput{ID: fixture.ID, Name: "X", IsDraft: !stored},
		)
		require.NoError(t, err)
		assert.Equal(t, stored, store.updatedDraft)
		assert.True(t, got.CanEdit)
		assert.Equal(t, 2, got.Servings)
	}
}

func TestRecipeUpdateKeepingDraft_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeRecipesStore{recipe: newRecipeFixture()}
	svc := &RecipeService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct // only the fields under test
	_, err := svc.UpdateKeepingDraft(
		t.Context(), "other", RecipeInput{ID: store.recipe.ID},
	)
	assert.Equal(t, http.StatusForbidden, httpStatus(t, err))
	assert.False(t, store.updated)
}

type stubVideos struct {
	video *Video
	err   error
	got   string
}

func (s *stubVideos) Fetch(_ context.Context, rawURL string) (*Video, error) {
	s.got = rawURL
	return s.video, s.err
}

func TestVideoFetch_PassesThrough(t *testing.T) {
	want := &Video{Platform: "youtube"} //nolint:exhaustruct // platform only
	//nolint:exhaustruct // only the fields under test
	//nolint:exhaustruct // only the video is stubbed
	stub := &stubVideos{video: want}
	svc := NewVideoService(stub)

	got, err := svc.Fetch(t.Context(), "https://youtu.be/x")
	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.Equal(t, "https://youtu.be/x", stub.got)

	stub.err = ErrVideoNotFound
	_, err = svc.Fetch(t.Context(), "x")
	assert.ErrorIs(t, err, ErrVideoNotFound)
}
