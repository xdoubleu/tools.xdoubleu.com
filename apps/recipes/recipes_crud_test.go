package recipes_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
)

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

// ── handler error paths ───────────────────────────────────────────────────────

func TestShareRecipe_InvalidRecipeUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/bad-id/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.ShareRecipeDto{ContactUserID: otherUserID})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestUnshareRecipe_InvalidRecipeUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/bad-id/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestUpdateOrDeleteRecipe_NotFoundUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/00000000-0000-0000-0000-000000000000")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"_action": {"update"}})
	//nolint:exhaustruct // minimal dto
	tReq.SetData(dtos.CreateRecipeDto{Name: "x", BaseServings: 2})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestUpdateOrDeleteRecipe_InvalidUUID(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/bad-id")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetQuery(url.Values{"_action": {"delete"}})
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

// ── listRecipesHandler — list populated ──────────────────────────────────────

func TestListRecipes_WithData(t *testing.T) {
	createTestRecipeWithIngredients(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet, "/recipes/list")
	assert.Equal(t, http.StatusOK, tReq.Do(t).StatusCode)
}

// ── editRecipeForm — with ingredients ────────────────────────────────────────

func TestEditRecipeForm_WithIngredients(t *testing.T) {
	id := createTestRecipeWithIngredients(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/"+id+"?edit=1")
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

// ── viewOrEditRecipeHandler — view shared recipe ──────────────────────────────

func TestViewRecipe_SharedWithUser(t *testing.T) {
	id := createTestRecipe(t)

	shareReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+id+"/share")
	shareReq.SetContentType(test.FormContentType)
	shareReq.SetFollowRedirect(false)
	shareReq.SetData(dtos.ShareRecipeDto{ContactUserID: otherUserID})
	require.Equal(t, http.StatusSeeOther, shareReq.Do(t).StatusCode)

	viewReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/"+id)
	assert.Equal(t, http.StatusOK, viewReq.Do(t).StatusCode)
}

// ── forbidden paths ───────────────────────────────────────────────────────────

func TestViewRecipe_ForbiddenForOtherUser(t *testing.T) {
	var recipeID string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ('other-user-000', 'Forbidden Recipe', '{}', 2)
		RETURNING id::text
	`).Scan(&recipeID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(t.Context(),
			`DELETE FROM recipes.recipes WHERE id = $1`, recipeID)
	})

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/"+recipeID)
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
}

func TestDeleteRecipe_ForbiddenForNonOwner(t *testing.T) {
	recipeID := insertOtherUserRecipe(t)
	body := strings.NewReader("_action=delete")
	req := httptest.NewRequest(http.MethodPost, "/recipes/"+recipeID, body)
	req.Header.Set("Content-Type", test.FormContentType)
	rr := httptest.NewRecorder()
	getRoutes().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestShareRecipe_ForbiddenForNonOwner(t *testing.T) {
	recipeID := insertOtherUserRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+recipeID+"/share")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.ShareRecipeDto{ContactUserID: otherUserID})
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
}

func TestUnshareRecipe_ForbiddenForNonOwner(t *testing.T) {
	recipeID := insertOtherUserRecipe(t)
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost,
		"/recipes/"+recipeID+"/share/"+otherUserID+"/delete")
	tReq.SetFollowRedirect(false)
	assert.Equal(t, http.StatusForbidden, tReq.Do(t).StatusCode)
}
