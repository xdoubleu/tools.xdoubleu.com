package recipes_test

import (
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

// ── New plan form ─────────────────────────────────────────────────────────────

func TestNewPlanForm_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/plans/new")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
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

func TestViewPlan_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/not-a-uuid")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

// ── Edit plan form ────────────────────────────────────────────────────────────

func TestEditPlanForm_OK(t *testing.T) {
	planID := createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/edit")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestEditPlanForm_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/edit")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestEditPlanForm_WithHideSlots(t *testing.T) {
	planID := createTestPlan(t)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/edit")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.UpdatePlanDto{
		Name:          "Slot Plan",
		ICalHideSlots: []string{"breakfast", "noon", "evening"},
		ICalHidePast:  true,
	})
	require.Equal(t, http.StatusSeeOther, tReq.Do(t).StatusCode)

	edit := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/edit")
	assert.Equal(t, http.StatusOK, edit.Do(t).StatusCode)
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

func TestUpdatePlan_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/edit")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	//nolint:exhaustruct // optional fields zero for this error path
	tReq.SetData(dtos.UpdatePlanDto{Name: "x"})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
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

func TestDeletePlan_NotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
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

func TestAddMeal_CustomName(t *testing.T) {
	planID := createTestPlan(t)
	today := time.Now().UTC().Format("2006-01-02")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	tReq.SetData(dtos.AddMealDto{
		MealDate:   today,
		MealSlot:   "evening",
		RecipeID:   "",
		CustomName: "Leftovers",
		Servings:   2,
	})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddMeal_InvalidRecipeUUID(t *testing.T) {
	planID := createTestPlan(t)
	today := time.Now().UTC().Format("2006-01-02")

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"offset": {"0"}})
	//nolint:exhaustruct // CustomName intentionally empty for this case
	tReq.SetData(dtos.AddMealDto{
		MealDate: today,
		MealSlot: "noon",
		RecipeID: "not-a-uuid",
		Servings: 2,
	})
	assert.Equal(t, http.StatusBadRequest, tReq.Do(t).StatusCode)
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

func TestDeleteMeal_InvalidPlanUUID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/bad-plan-id/meals/00000000-0000-0000-0000-000000000000/delete",
	)
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestDeleteMeal_InvalidMealUUID(t *testing.T) {
	planID := createTestPlan(t)

	tReq := test.CreateRequestTester(
		getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals/not-a-uuid/delete",
	)
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestDeleteMeal_ForbiddenNoEditAccess(t *testing.T) {
	planID := insertSharedPlan(t, false)
	recipeID := createTestRecipe(t)

	today := time.Now().UTC().Format("2006-01-02")
	var mealID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO recipes.plan_meals (plan_id, meal_date, meal_slot, recipe_id, servings)
		VALUES ($1, $2, 'noon', $3, 2)
		RETURNING id::text`,
		planID, today, recipeID,
	).Scan(&mealID)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/meals/"+mealID+"/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
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

func TestShoppingList_WithMeals(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipeWithIngredients(t)
	addTestMeal(t, planID, recipeID)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/shopping")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

func TestShoppingList_TxtFormat_WithMeals(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipeWithIngredients(t)
	addTestMeal(t, planID, recipeID)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID+"/shopping")
	tReq.SetQuery(url.Values{"format": {"txt"}})
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
	assert.Contains(t, rs.Header.Get("Content-Type"), "text/plain")
}

// ── viewPlanHandler — all three meal slots filled ─────────────────────────────

func TestViewPlan_AllSlotsPopulated(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipeWithIngredients(t)

	addMealSlot(t, planID, recipeID, "breakfast")
	addMealSlot(t, planID, recipeID, "noon")
	addMealSlot(t, planID, recipeID, "evening")

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID)
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── listPlansHandler — list populated ────────────────────────────────────────

func TestListPlans_WithData(t *testing.T) {
	createTestPlan(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/plans")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
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

func TestSharePlan_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/bad-id/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SharePlanDto{ContactUserID: otherUserID, CanEdit: false})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestSharePlan_ForbiddenForNonOwner(t *testing.T) {
	planID := insertSharedPlan(t, true)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.SharePlanDto{ContactUserID: otherUserID, CanEdit: false})
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
}

// ── Unshare plan ──────────────────────────────────────────────────────────────

func TestUnsharePlan_Redirects(t *testing.T) {
	planID := createTestPlan(t)

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

func TestUnsharePlan_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/bad-id/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestUnsharePlan_ForbiddenForNonOwner(t *testing.T) {
	planID := insertSharedPlan(t, true)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
}

// ── viewPlanHandler — view shared plan ───────────────────────────────────────

func TestViewPlan_SharedWithUser(t *testing.T) {
	planID := createTestPlan(t)

	shareReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/plans/"+planID+"/share")
	shareReq.SetContentType(test.FormContentType)
	shareReq.SetFollowRedirect(false)
	shareReq.SetData(dtos.SharePlanDto{ContactUserID: otherUserID, CanEdit: true})
	require.Equal(t, http.StatusSeeOther, shareReq.Do(t).StatusCode)

	viewReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/"+planID)
	assert.Equal(t, http.StatusOK, viewReq.Do(t).StatusCode)
}
