package recipes_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
)

// ── Recipe list ───────────────────────────────────────────────────────────────

func TestListRecipes_OK(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes")
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
	assert.Equal(t, "/recipes", rs.Header.Get("Location"))
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
		Name:      "Test Week",
		StartDate: "2026-04-28",
		EndDate:   "2026-05-04",
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
	// Create a plan and retrieve its ical_token from the DB
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/plans/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{
		Name:      "iCal Test Plan",
		StartDate: "2026-05-05",
		EndDate:   "2026-05-11",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	planPath := rs.Header.Get("Location")
	planID := strings.TrimPrefix(planPath, "/recipes/plans/")

	// Query the ical_token from DB
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

// ── Shopping list ─────────────────────────────────────────────────────────────

func TestShoppingList_PlanNotFound(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/plans/00000000-0000-0000-0000-000000000000/shopping")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}
