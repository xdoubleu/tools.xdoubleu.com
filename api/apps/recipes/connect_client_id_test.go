package recipes_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func userCtx() context.Context {
	return contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
}

func deleteRecipeAfter(t *testing.T, id string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(), "DELETE FROM recipes.recipes WHERE id = $1", id,
		)
	})
}

func TestCreateRecipe_ClientIDRepeatIsIdempotent(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	id := uuid.NewString()
	deleteRecipeAfter(t, id)
	req := &recipesv1.CreateRecipeRequest{
		Id:                id,
		Name:              "Offline soup",
		Steps:             []string{"Simmer"},
		BaseServings:      2,
		IngredientNames:   []string{"Water", "Salt"},
		IngredientAmounts: []float64{1, 0.5},
		IngredientUnits:   []string{"l", "tsp"},
	}

	first, err := client.CreateRecipe(userCtx(), connect.NewRequest(req))
	require.NoError(t, err)
	second, err := client.CreateRecipe(userCtx(), connect.NewRequest(req))
	require.NoError(t, err)

	assert.Equal(t, id, first.Msg.Recipe.Id)
	assert.Equal(t, id, second.Msg.Recipe.Id)

	got, err := client.GetRecipe(
		userCtx(), connect.NewRequest(&recipesv1.GetRecipeRequest{Id: id}),
	)
	require.NoError(t, err)
	assert.Len(t, got.Msg.Recipe.Ingredients, 2)

	var count int
	require.NoError(t, testDB.QueryRow(
		context.Background(),
		"SELECT count(*) FROM recipes.recipes WHERE id = $1", id,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestCreateRecipe_ClientIDOwnedByAnotherFamily(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	var id string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, family_id, name, instructions, base_servings)
		VALUES ('someone-else', gen_random_uuid(), 'Theirs', '', 2)
		RETURNING id::text`,
	).Scan(&id))
	deleteRecipeAfter(t, id)

	_, err := client.CreateRecipe(userCtx(), connect.NewRequest(
		&recipesv1.CreateRecipeRequest{Id: id, Name: "Mine", BaseServings: 2},
	))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestCreateRecipe_InvalidClientID(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	_, err := client.CreateRecipe(userCtx(), connect.NewRequest(
		&recipesv1.CreateRecipeRequest{Id: "nope", Name: "x", BaseServings: 2},
	))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestCreateRecipe_ReplayKeepsLaterEdits(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	id := uuid.NewString()
	deleteRecipeAfter(t, id)
	create := &recipesv1.CreateRecipeRequest{
		Id:                id,
		Name:              "Draft stew",
		BaseServings:      2,
		IngredientNames:   []string{"Beans"},
		IngredientAmounts: []float64{1},
		IngredientUnits:   []string{"can"},
	}
	_, err := client.CreateRecipe(userCtx(), connect.NewRequest(create))
	require.NoError(t, err)

	_, err = client.UpdateRecipe(
		userCtx(),
		connect.NewRequest(&recipesv1.UpdateRecipeRequest{
			Id:                id,
			Name:              "Stew",
			BaseServings:      2,
			IngredientNames:   []string{"Lentils", "Carrot"},
			IngredientAmounts: []float64{2, 1},
			IngredientUnits:   []string{"cup", ""},
		}),
	)
	require.NoError(t, err)

	replayed, err := client.CreateRecipe(userCtx(), connect.NewRequest(create))
	require.NoError(t, err)
	assert.Equal(t, "Stew", replayed.Msg.Recipe.Name)
	assert.Len(t, replayed.Msg.Recipe.Ingredients, 2)
}
