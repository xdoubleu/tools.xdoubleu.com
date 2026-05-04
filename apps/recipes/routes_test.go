package recipes_test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
)

const otherUserID = "00000000-0000-0000-0000-000000000002"

// ── Helpers ───────────────────────────────────────────────────────────────────

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

// ── Recipe list ───────────────────────────────────────────────────────────────

func TestListRecipes_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestListRecipesPage_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/list")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Recipe form ───────────────────────────────────────────────────────────────

func TestNewRecipeForm_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/new")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestEditRecipeForm_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/00000000-0000-0000-0000-000000000000?edit=1")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestEditRecipeForm_OK(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/"+id+"?edit=1")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Create recipe ─────────────────────────────────────────────────────────────

func TestCreateRecipe_Redirects(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct //other fields optional
	tReq.SetData(dtos.CreateRecipeDto{
		Name:         "Test Pasta",
		BaseServings: 2,
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/list", rs.Header.Get("Location"))
}

// ── View recipe ───────────────────────────────────────────────────────────────

func TestViewRecipe_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/00000000-0000-0000-0000-000000000000")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestViewRecipe_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/not-a-uuid")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestViewRecipe_OK(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/"+id)
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestViewRecipe_WithServings(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/"+id)
	tReq.SetQuery(url.Values{"servings": {"4"}})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Update recipe ─────────────────────────────────────────────────────────────

func TestUpdateRecipe_Redirects(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/"+id)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"_action": {"update"}})
	//nolint:exhaustruct //ingredient fields optional
	tReq.SetData(dtos.CreateRecipeDto{
		Name:         "Updated Pasta",
		Steps:        []string{"Boil more water."},
		BaseServings: 4,
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/"+id, rs.Header.Get("Location"))
}

// ── Delete recipe ─────────────────────────────────────────────────────────────

func TestDeleteRecipe_Redirects(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/"+id)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"_action": {"delete"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/list", rs.Header.Get("Location"))
}

// ── Plan list ─────────────────────────────────────────────────────────────────

func TestListPlans_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/plans")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Create plan ───────────────────────────────────────────────────────────────

func TestCreatePlan_Redirects(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/plans/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{
		Name: "Test Week",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.True(t, strings.HasPrefix(rs.Header.Get("Location"), "/recipes/plans/"))
}

// ── View plan ─────────────────────────────────────────────────────────────────

func TestViewPlan_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/00000000-0000-0000-0000-000000000000")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestViewPlan_OK(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID)
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestViewPlan_WithOffset(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID)
	tReq.SetQuery(url.Values{"offset": {"1"}})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Edit plan form ────────────────────────────────────────────────────────────

func TestEditPlanForm_OK(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/edit")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Update plan ───────────────────────────────────────────────────────────────

func TestUpdatePlan_Redirects(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/edit")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{Name: "Updated Week"})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/plans/"+planID, rs.Header.Get("Location"))
}

// ── Delete plan ───────────────────────────────────────────────────────────────

func TestDeletePlan_Redirects(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/delete")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/plans", rs.Header.Get("Location"))
}

// ── Add meal ──────────────────────────────────────────────────────────────────

func TestAddMeal_Redirects(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
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
		Servings: 3,
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(
		t,
		"/recipes/plans/"+planID+"?offset=0",
		rs.Header.Get("Location"),
	)
}

func TestAddMeal_InvalidPlan(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/meals",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	//nolint:exhaustruct // CustomName not needed when RecipeID is set
	tReq.SetData(dtos.AddMealDto{
		MealDate: time.Now().UTC().Format("2006-01-02"),
		MealSlot: "noon",
		RecipeID: "00000000-0000-0000-0000-000000000001",
		Servings: 2,
	})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

// ── Delete meal ───────────────────────────────────────────────────────────────

func TestDeleteMeal_Redirects(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
	mealID := addTestMeal(t, planID, recipeID)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals/"+mealID+"/delete",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/plans/"+planID+"?offset=0", rs.Header.Get("Location"))
}

// ── Shopping list ─────────────────────────────────────────────────────────────

func TestShoppingList_PlanNotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/shopping")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestShoppingList_OK(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/shopping")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestShoppingList_TxtFormat(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/shopping")
	tReq.SetQuery(url.Values{"format": {"txt"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
	assert.Contains(t, rs.Header.Get("Content-Type"), "text/plain")
}

// ── iCal feed ─────────────────────────────────────────────────────────────────

func TestICalFeed_InvalidToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/ical/not-a-uuid.ics")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestICalFeed_UnknownToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/ical/00000000-0000-0000-0000-000000000099.ics")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestICalFeed_ValidToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/plans/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{
		Name: "iCal Test Plan",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	planPath := rs.Header.Get("Location")
	planID := strings.TrimPrefix(planPath, "/recipes/plans/")

	var icalToken string
	err := testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	icalReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	icalRS := icalReq.Do(t)
	assert.Equal(t, http.StatusOK, icalRS.StatusCode)
	assert.Equal(t, "text/calendar; charset=utf-8", icalRS.Header.Get("Content-Type"))
}

func TestICalFeed_WithMeals(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
	addTestMeal(t, planID, recipeID)

	var icalToken string
	err := testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "BEGIN:VEVENT")
	assert.Contains(t, bodyStr, "DTSTART:")
	assert.Contains(t, bodyStr, "DTEND:")
	assert.Contains(t, bodyStr, "DTSTAMP:")
	assert.Contains(t, bodyStr, "SUMMARY:Noon – Test Pasta")
}

// ── iCal feed slot / past filtering ──────────────────────────────────────────

func TestICalFeed_HidesSlot(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
	addTestMeal(t, planID, recipeID)

	_, err := testDB.Exec(
		t.Context(),
		`UPDATE recipes.plans SET ical_hide_slots = '{noon}' WHERE id = $1`,
		planID,
	)
	require.NoError(t, err)

	var icalToken string
	err = testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "BEGIN:VEVENT")
}

func TestICalFeed_HidesPastMeals(t *testing.T) {
	planID := createTestPlan(t)

	_, err := testDB.Exec(
		t.Context(),
		`UPDATE recipes.plans SET ical_hide_past = true WHERE id = $1`,
		planID,
	)
	require.NoError(t, err)

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_, err = testDB.Exec(
		t.Context(),
		`INSERT INTO recipes.plan_meals (plan_id, meal_date, meal_slot, custom_name, servings)
		 VALUES ($1, $2, 'noon', 'Yesterday meal', 2)`,
		planID,
		yesterday,
	)
	require.NoError(t, err)

	var icalToken string
	err = testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "BEGIN:VEVENT")
}

func TestUpdatePlan_WithICalSettings(t *testing.T) {
	planID := createTestPlan(t)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/edit")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdatePlanDto{
		Name:          "Updated Week",
		ICalHideSlots: []string{"breakfast"},
		ICalHidePast:  true,
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	var hideSlots []string
	var hidePast bool
	err := testDB.QueryRow(
		t.Context(),
		`SELECT ical_hide_slots, ical_hide_past FROM recipes.plans WHERE id = $1`,
		planID,
	).Scan(&hideSlots, &hidePast)
	require.NoError(t, err)
	assert.Equal(t, []string{"breakfast"}, hideSlots)
	assert.True(t, hidePast)
}

// ── New plan form ─────────────────────────────────────────────────────────────

func TestNewPlanForm_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/plans/new")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── Share recipe ──────────────────────────────────────────────────────────────

func TestShareRecipe_Redirects(t *testing.T) {
	id := createTestRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+id+"/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.ShareRecipeDto{ContactUserID: otherUserID})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/"+id, rs.Header.Get("Location"))
}

// ── Unshare recipe ────────────────────────────────────────────────────────────

func TestUnshareRecipe_Redirects(t *testing.T) {
	id := createTestRecipe(t)

	// Share first so there is something to remove.
	shareReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+id+"/share")
	shareReq.SetContentType(test.FormContentType)
	shareReq.SetFollowRedirect(false)
	shareReq.SetData(dtos.ShareRecipeDto{ContactUserID: otherUserID})
	require.Equal(t, http.StatusSeeOther, shareReq.Do(t).StatusCode)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+id+"/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/"+id, rs.Header.Get("Location"))
}

// ── Share plan ────────────────────────────────────────────────────────────────

func TestSharePlan_Redirects(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SharePlanDto{ContactUserID: otherUserID, CanEdit: false})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/plans/"+planID, rs.Header.Get("Location"))
}

// ── Unshare plan ──────────────────────────────────────────────────────────────

func TestUnsharePlan_Redirects(t *testing.T) {
	planID := createTestPlan(t)

	// Share first so there is something to remove.
	shareReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share")
	shareReq.SetContentType(test.FormContentType)
	shareReq.SetFollowRedirect(false)
	shareReq.SetData(dtos.SharePlanDto{ContactUserID: otherUserID, CanEdit: false})
	require.Equal(t, http.StatusSeeOther, shareReq.Do(t).StatusCode)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/recipes/plans/"+planID, rs.Header.Get("Location"))
}

// ── Recipe with ingredients ───────────────────────────────────────────────────

func TestViewRecipe_WithIngredients(t *testing.T) {
	id := createTestRecipeWithIngredients(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/"+id)
	tReq.SetQuery(url.Values{"servings": {"4"}})
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestUpdateRecipe_WithIngredients(t *testing.T) {
	id := createTestRecipeWithIngredients(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/"+id)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"_action": {"update"}})
	tReq.SetData(dtos.CreateRecipeDto{
		Name:              "Updated Pasta",
		Steps:             []string{"New instructions."},
		BaseServings:      4,
		IngredientNames:   []string{"pasta", "sauce"},
		IngredientAmounts: []string{"300", "150"},
		IngredientUnits:   []string{"g", "ml"},
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}
