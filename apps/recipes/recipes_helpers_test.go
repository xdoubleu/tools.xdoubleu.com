package recipes_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
)

const otherUserID = "00000000-0000-0000-0000-000000000002"

func createTestRecipeWithIngredients(t *testing.T) string {
	t.Helper()
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreateRecipeDto{
		Name:              "Pasta",
		Steps:             []string{"Boil water, cook pasta."},
		BaseServings:      2,
		IngredientNames:   []string{"pasta"},
		IngredientAmounts: []string{"200"},
		IngredientUnits:   []string{"g"},
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(
		t.Context(),
		`SELECT id::text FROM recipes.recipes
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func createTestRecipe(t *testing.T) string {
	t.Helper()
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct //ingredient fields optional
	tReq.SetData(dtos.CreateRecipeDto{
		Name:         "Test Pasta",
		Steps:        []string{"Boil water."},
		BaseServings: 2,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var id string
	err := testDB.QueryRow(
		t.Context(),
		`SELECT id::text FROM recipes.recipes
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func createTestPlan(t *testing.T) string {
	t.Helper()
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/plans/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{Name: "Test Week"})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)
	return strings.TrimPrefix(rs.Header.Get("Location"), "/recipes/plans/")
}

func addTestMeal(t *testing.T, planID, recipeID string) string {
	t.Helper()
	today := time.Now().UTC().Format("2006-01-02")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	//nolint:exhaustruct // CustomName not needed when RecipeID is set
	tReq.SetData(dtos.AddMealDto{
		MealDate: today,
		MealSlot: "noon",
		RecipeID: recipeID,
		Servings: 2,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var mealID string
	err := testDB.QueryRow(
		t.Context(),
		`SELECT id::text FROM recipes.plan_meals
		 WHERE plan_id = $1 AND meal_date = $2 AND meal_slot = 'noon'`,
		planID, today,
	).Scan(&mealID)
	require.NoError(t, err)
	return mealID
}

func addMealSlot(t *testing.T, planID, recipeID, slot string) {
	t.Helper()
	today := time.Now().UTC().Format("2006-01-02")
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	//nolint:exhaustruct // CustomName not needed when RecipeID is set
	tReq.SetData(dtos.AddMealDto{
		MealDate: today,
		MealSlot: slot,
		RecipeID: recipeID,
		Servings: 2,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func insertOtherUserRecipe(t *testing.T) string {
	t.Helper()
	var recipeID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ('other-recipe-owner', 'Other Recipe', '{}', 2)
		RETURNING id::text
	`).Scan(&recipeID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(t.Context(),
			`DELETE FROM recipes.recipes WHERE id = $1`, recipeID)
	})
	return recipeID
}

func insertSharedPlan(t *testing.T, canEdit bool) string {
	t.Helper()
	var planID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO recipes.plans (owner_user_id, name)
		VALUES ('other-plan-owner', 'Shared Plan')
		RETURNING id::text
	`).Scan(&planID)
	require.NoError(t, err)
	_, err = testDB.Exec(t.Context(), `
		INSERT INTO recipes.plan_access (plan_id, user_id, can_edit)
		VALUES ($1, $2, $3)`,
		planID, userID, canEdit,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(t.Context(),
			`DELETE FROM recipes.plans WHERE id = $1`, planID)
	})
	return planID
}
