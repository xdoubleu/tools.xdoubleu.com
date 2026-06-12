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

func TestShareRecipeBook_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.ShareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.ShareRecipeBookRequest{
			ContactUserId: "share-success-target",
			CanEdit:       true,
		}),
	)
	require.NoError(t, err)

	shares, err := client.ListRecipeBookShares(
		ctx,
		connect.NewRequest(&recipesv1.ListRecipeBookSharesRequest{}),
	)
	require.NoError(t, err)
	var found *recipesv1.RecipeBookShare
	for _, s := range shares.Msg.Shares {
		if s.UserId == "share-success-target" {
			found = s
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.CanEdit)
}

func TestShareRecipeBook_RejectsSelf(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.ShareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.ShareRecipeBookRequest{ContactUserId: userID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestGetRecipe_NoBookAccessDenied(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	// A recipe owned by someone else with no recipe-book grant is inaccessible.
	var recipeID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ('no-grant-owner', 'Hidden Dish', 'mix', 2)
		RETURNING id::text`,
	).Scan(&recipeID)
	require.NoError(t, err)

	_, err = client.GetRecipe(
		ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.Error(t, err)
}

func TestShareRecipeBook_RequiresContact(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.ShareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.ShareRecipeBookRequest{ContactUserId: ""}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUnshareRecipeBook_RequiresTargetUserID(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.UnshareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.UnshareRecipeBookRequest{TargetUserId: ""}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUnshareRecipeBook_Success(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	_, err := client.ShareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.ShareRecipeBookRequest{
			ContactUserId: "unshare-target",
		}),
	)
	require.NoError(t, err)

	_, err = client.UnshareRecipeBook(
		ctx,
		connect.NewRequest(&recipesv1.UnshareRecipeBookRequest{
			TargetUserId: "unshare-target",
		}),
	)
	require.NoError(t, err)

	shares, err := client.ListRecipeBookShares(
		ctx,
		connect.NewRequest(&recipesv1.ListRecipeBookSharesRequest{}),
	)
	require.NoError(t, err)
	for _, s := range shares.Msg.Shares {
		assert.NotEqual(t, "unshare-target", s.UserId)
	}
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

// TestRecipeBookShare_GrantsAccess sets up a recipe owned by a different user
// and a recipe-book grant to userID directly in the DB (the mock auth always
// authenticates the server as userID, so the recipient side must be staged in
// the database), then exercises the access paths through the handler as userID.
func TestRecipeBookShare_GrantsAccess(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	const otherOwner = "book-owner-2"
	var recipeID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ($1, 'Other Owner Dish', 'mix', 2)
		RETURNING id::text`,
		otherOwner,
	).Scan(&recipeID)
	require.NoError(t, err)

	_, err = testDB.Exec(context.Background(), `
		INSERT INTO recipes.recipebook_access (owner_user_id, user_id, can_edit)
		VALUES ($1, $2, TRUE)`,
		otherOwner, userID,
	)
	require.NoError(t, err)

	// ListRecipes surfaces the shared owner's recipe.
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
	assert.True(t, inList, "shared recipe should appear in the recipient's list")

	// GetRecipe grants access with edit rights but not ownership.
	getResp, err := client.GetRecipe(
		ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.NoError(t, err)
	assert.False(t, getResp.Msg.IsOwner)
	assert.True(t, getResp.Msg.CanEdit)

	// An edit-sharer may update the recipe; ownership stays with the creator.
	_, err = client.UpdateRecipe(
		ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id: recipeID, Name: "Edited By Sharer",
			Steps: []string{"new"}, BaseServings: 3,
		}),
	)
	require.NoError(t, err)
}

// TestRecipeBookShare_ViewOnlyCannotEdit stages a view-only grant and confirms
// the recipient can read but not update the recipe.
func TestRecipeBookShare_ViewOnlyCannotEdit(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	const otherOwner = "book-owner-3"
	var recipeID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ($1, 'View Only Dish', 'mix', 2)
		RETURNING id::text`,
		otherOwner,
	).Scan(&recipeID)
	require.NoError(t, err)

	_, err = testDB.Exec(context.Background(), `
		INSERT INTO recipes.recipebook_access (owner_user_id, user_id, can_edit)
		VALUES ($1, $2, FALSE)`,
		otherOwner, userID,
	)
	require.NoError(t, err)

	getResp, err := client.GetRecipe(
		ctx, connect.NewRequest(&recipesv1.GetRecipeRequest{Id: recipeID}),
	)
	require.NoError(t, err)
	assert.False(t, getResp.Msg.CanEdit)

	_, err = client.UpdateRecipe(
		ctx, connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id: recipeID, Name: "Should Fail",
			Steps: []string{"x"}, BaseServings: 2,
		}),
	)
	require.Error(t, err)
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
