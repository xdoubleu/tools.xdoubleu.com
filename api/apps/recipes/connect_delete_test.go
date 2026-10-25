package recipes_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// A meal-plan row blocks the delete and the recipe stays. This package's
// migrations create the plan tables in the recipes schema; in production the
// mealplans app moves them, keeping the same constraint names.
func TestDeleteRecipe_OnMealPlanIsBlocked(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreateRecipe(ctx, &recipesv1.CreateRecipeRequest{
		Name:         "Planned Recipe",
		Steps:        []string{"step"},
		BaseServings: 2,
	})
	require.NoError(t, err)
	recipeID := createResp.Recipe.Id

	var planID string
	require.NoError(t, testDB.QueryRow(
		context.Background(),
		`INSERT INTO recipes.plans (owner_user_id, name)
		 VALUES ($1, 'Plan') RETURNING id::text`,
		userID,
	).Scan(&planID))
	// Deleting the plan cascades to its meals; the tests share one database and
	// the blocked recipe could not be removed otherwise.
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(), `DELETE FROM recipes.plans WHERE id = $1`, planID,
		)
		_, _ = client.DeleteRecipe(ctx, &recipesv1.DeleteRecipeRequest{Id: recipeID})
	})
	_, err = testDB.Exec(
		context.Background(),
		`INSERT INTO recipes.plan_meals (plan_id, meal_date, meal_slot, recipe_id)
		 VALUES ($1, CURRENT_DATE, 'noon', $2)`,
		planID, recipeID,
	)
	require.NoError(t, err)

	_, err = client.DeleteRecipe(ctx, &recipesv1.DeleteRecipeRequest{Id: recipeID})
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connectErr(err).Code())
	assert.Equal(
		t,
		"This recipe is used in a meal plan; remove it there first.",
		connectErr(err).Message(),
	)

	getResp, err := client.GetRecipe(ctx, &recipesv1.GetRecipeRequest{Id: recipeID})
	require.NoError(t, err)
	assert.Equal(t, recipeID, getResp.Recipe.Id)
}
