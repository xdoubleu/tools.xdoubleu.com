package recipes_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func setupRecipesClient(handler http.Handler) recipesv1connect.RecipesServiceClient {
	ts := httptest.NewServer(handler)
	return recipesv1connect.NewRecipesServiceClient(http.DefaultClient, ts.URL)
}

func contextWithUser(ctx context.Context, user *sharedmodels.User) context.Context {
	return context.WithValue(ctx, constants.UserContextKey, user)
}

func connectErr(err error) *connect.Error {
	target := &connect.Error{}
	_ = errors.As(err, &target)
	return target
}

func TestListRecipes_Empty(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	resp, err := client.ListRecipes(
		ctx,
		connect.NewRequest(&recipesv1.ListRecipesRequest{}),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, len(resp.Msg.Recipes))
}

func TestCreateRecipe_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	resp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:              "Pasta Carbonara",
			Steps:             []string{"Boil water", "Cook pasta", "Mix eggs"},
			BaseServings:      4,
			IngredientNames:   []string{"Pasta", "Eggs", "Bacon"},
			IngredientAmounts: []float64{400, 4, 200},
			IngredientUnits:   []string{"g", "", "g"},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Pasta Carbonara", resp.Msg.Recipe.Name)
	assert.Equal(t, int32(4), resp.Msg.Recipe.BaseServings)
	assert.Equal(t, 3, len(resp.Msg.Recipe.Ingredients))
	assert.Equal(t, userID, resp.Msg.Recipe.UserId)
}

func TestGetRecipe_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:              "Test Recipe",
			Steps:             []string{"Step 1", "Step 2"},
			BaseServings:      2,
			IngredientNames:   []string{"Ingredient 1"},
			IngredientAmounts: []float64{1},
			IngredientUnits:   []string{"cup"},
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{
			Id: createResp.Msg.Recipe.Id,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Test Recipe", getResp.Msg.Recipe.Name)
	assert.Equal(t, int32(2), getResp.Msg.Servings)
	assert.True(t, getResp.Msg.IsOwner)
}

func TestGetRecipe_WithServingScale(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:              "Scaling Test",
			Steps:             []string{"Mix well"},
			BaseServings:      2,
			IngredientNames:   []string{"Flour"},
			IngredientAmounts: []float64{2},
			IngredientUnits:   []string{"cups"},
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{
			Id: createResp.Msg.Recipe.Id, Servings: 4,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(4), getResp.Msg.Servings)
	assert.Equal(t, 1, len(getResp.Msg.ScaledIngredients))
	assert.Equal(t, "4", getResp.Msg.ScaledIngredients[0].Amount)
}

// TestGetRecipe_OtherFamilyDenied stages a recipe belonging to an unrelated
// family (a fresh random family_id, not userID's) directly in the database
// and confirms userID cannot access it.
func TestGetRecipe_OtherFamilyDenied(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	var recipeID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, family_id, name, instructions, base_servings)
		VALUES ('no-access-owner', gen_random_uuid(), 'Hidden Dish', 'mix', 2)
		RETURNING id::text`,
	).Scan(&recipeID)
	require.NoError(t, err)

	_, err = client.GetRecipe(
		ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connectErr(err).Code())
}

func TestGetRecipe_NotFound(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.GetRecipe(ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{
		Id: uuid.New().String(),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestUpdateRecipe_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:              "Original Name",
			Steps:             []string{"Do something"},
			BaseServings:      2,
			IngredientNames:   []string{"Ingredient"},
			IngredientAmounts: []float64{1},
			IngredientUnits:   []string{""},
		}),
	)
	require.NoError(t, err)
	recipeID := createResp.Msg.Recipe.Id

	_, err = client.UpdateRecipe(ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
		Id:                recipeID,
		Name:              "Updated Name",
		Steps:             []string{"Do something else"},
		BaseServings:      4,
		IngredientNames:   []string{"Ingredient"},
		IngredientAmounts: []float64{2},
		IngredientUnits:   []string{""},
	}))
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{
			Id: recipeID,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", getResp.Msg.Recipe.Name)
	assert.Equal(t, int32(4), getResp.Msg.Recipe.BaseServings)
}

func TestDeleteRecipe_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:         "To Delete",
			Steps:        []string{"Delete me"},
			BaseServings: 2,
		}),
	)
	require.NoError(t, err)
	recipeID := createResp.Msg.Recipe.Id

	_, err = client.DeleteRecipe(ctx, connect.NewRequest(&recipesv1.DeleteRecipeRequest{
		Id: recipeID,
	}))
	require.NoError(t, err)

	_, err = client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestCreateRecipe_WithBatchServings(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	batchServings := int32(10)
	resp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:          "Batch Chili",
			Steps:         []string{"Cook everything"},
			BaseServings:  2,
			BatchServings: &batchServings,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Recipe.BatchServings)
	assert.Equal(t, int32(10), *resp.Msg.Recipe.BatchServings)
}

func TestUpdateRecipe_WithBatchServings(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:         "Batch Recipe",
			Steps:        []string{"Step 1"},
			BaseServings: 2,
		}),
	)
	require.NoError(t, err)
	assert.Nil(t, createResp.Msg.Recipe.BatchServings)

	recipeID := createResp.Msg.Recipe.Id
	batchServings := int32(8)
	_, err = client.UpdateRecipe(ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
		Id:            recipeID,
		Name:          "Batch Recipe",
		Steps:         []string{"Step 1"},
		BaseServings:  2,
		BatchServings: &batchServings,
	}))
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Recipe.BatchServings)
	assert.Equal(t, int32(8), *getResp.Msg.Recipe.BatchServings)
}

func TestUpdateRecipe_ClearBatchServings(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	batchServings := int32(12)
	createResp, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name:          "Was Batch",
			Steps:         []string{"Step"},
			BaseServings:  2,
			BatchServings: &batchServings,
		}),
	)
	require.NoError(t, err)
	recipeID := createResp.Msg.Recipe.Id

	_, err = client.UpdateRecipe(ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
		Id:           recipeID,
		Name:         "Was Batch",
		Steps:        []string{"Step"},
		BaseServings: 2,
		// BatchServings intentionally omitted to clear it
	}))
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Recipe.BatchServings)
}

func TestListRecipes_WithItems(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.CreateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.CreateRecipeRequest{
			Name: "Listed Recipe", Steps: []string{"step"}, BaseServings: 2,
		}),
	)
	require.NoError(t, err)

	resp, err := client.ListRecipes(
		ctx,
		connect.NewRequest(&recipesv1.ListRecipesRequest{}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Recipes)
}

// TestListRecipes_Pagination verifies Limit/Offset bound the page and
// HasMore reflects whether more rows exist beyond it. Recipe names are
// prefixed to sort after every other recipe this shared-fixture suite may
// have already created for userID, so the three new rows land contiguously
// at the end of the alphabetical order used by ListForUser.
func TestListRecipes_Pagination(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ID: userID}, //nolint:exhaustruct // only ID needed
	)

	baseline, err := client.ListRecipes(
		ctx, connect.NewRequest(&recipesv1.ListRecipesRequest{Limit: 1000}),
	)
	require.NoError(t, err)
	existing := int32(len(baseline.Msg.Recipes))

	for _, name := range []string{
		"zzz_paginated_one", "zzz_paginated_two", "zzz_paginated_three",
	} {
		_, err = client.CreateRecipe(
			ctx,
			connect.NewRequest(&recipesv1.CreateRecipeRequest{
				Name: name, Steps: []string{"step"}, BaseServings: 2,
			}),
		)
		require.NoError(t, err)
	}

	firstPage, err := client.ListRecipes(
		ctx, connect.NewRequest(&recipesv1.ListRecipesRequest{
			Limit: 2, Offset: existing,
		}),
	)
	require.NoError(t, err)
	assert.Len(t, firstPage.Msg.Recipes, 2)
	assert.True(t, firstPage.Msg.HasMore)

	secondPage, err := client.ListRecipes(
		ctx, connect.NewRequest(&recipesv1.ListRecipesRequest{
			Limit: 2, Offset: existing + 2,
		}),
	)
	require.NoError(t, err)
	assert.Len(t, secondPage.Msg.Recipes, 1)
	assert.False(t, secondPage.Msg.HasMore)
}

func TestDeleteRecipe_NotFound(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.DeleteRecipe(
		ctx,
		connect.NewRequest(&recipesv1.DeleteRecipeRequest{
			Id: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

// TestFamilyMember_GrantsAccess creates a recipe owned by another user, joins
// them into userID's family directly in the DB (the mock auth always
// authenticates the server as userID, so the family membership must be
// staged directly), then exercises the access paths through the handler as
// userID.
func TestFamilyMember_GrantsAccess(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	const familyMember = "family-member-1"
	familyID, err := familyRepo.EnsureFamily(context.Background(), userID)
	require.NoError(t, err)
	_, err = testDB.Exec(context.Background(), `
		INSERT INTO global.family_members (user_id, family_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET family_id = EXCLUDED.family_id`,
		familyMember, familyID,
	)
	require.NoError(t, err)

	var recipeID string
	err = testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, family_id, name, instructions, base_servings)
		VALUES ($1, $2, 'Family Member Dish', 'mix', 2)
		RETURNING id::text`,
		familyMember, familyID,
	).Scan(&recipeID)
	require.NoError(t, err)

	// ListRecipes surfaces the family member's recipe.
	listResp, err := client.ListRecipes(
		ctx, connect.NewRequest(&recipesv1.ListRecipesRequest{}),
	)
	require.NoError(t, err)
	var inList bool
	for _, r := range listResp.Msg.Recipes {
		if r.Id == recipeID {
			inList = true
		}
	}
	assert.True(t, inList, "family member's recipe should appear in userID's list")

	// GetRecipe grants full edit rights but ownership display stays with the
	// creator.
	getResp, err := client.GetRecipe(
		ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.NoError(t, err)
	assert.False(t, getResp.Msg.IsOwner)
	assert.True(t, getResp.Msg.CanEdit)

	// Any family member may update the recipe; ownership stays with the
	// creator.
	_, err = client.UpdateRecipe(
		ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id: recipeID, Name: "Edited By Family Member",
			Steps: []string{"new"}, BaseServings: 3,
		}),
	)
	require.NoError(t, err)
}

func TestUpdateRecipe_NotFound(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.UpdateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id: uuid.New().
				String(),
			Name: "ghost", Steps: []string{"s"}, BaseServings: 1,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestGetRecipe_InvalidID(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.GetRecipe(
		ctx,
		connect.NewRequest(&recipesv1.GetRecipeRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUpdateRecipe_InvalidID(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.UpdateRecipe(
		ctx,
		connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id: "not-a-uuid", Name: "x", Steps: []string{"s"}, BaseServings: 1,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestDeleteRecipe_InvalidID(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.DeleteRecipe(
		ctx,
		connect.NewRequest(&recipesv1.DeleteRecipeRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
