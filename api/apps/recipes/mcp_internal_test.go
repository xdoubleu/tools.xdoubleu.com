package recipes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMCPTools_CreateRecipeIsDraft: recipes_create_recipe persists through the
// same service layer as the Connect RPCs and always lands as a draft, even
// though its arguments carry no is_draft flag.
func TestMCPTools_CreateRecipeIsDraft(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pg := testhelper.ConnectTestDB(cfg.DBDsn)

	const mcpUserID = "mcp-recipes-user"
	app := New(
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		sharedrepositories.NewFamilyRepository(pg),
	)
	h := &recipesConnectHandler{app: app}

	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: mcpUserID, AppAccess: []string{mcpAppName}},
	)

	batch := int32(8)
	createdMsg, err := h.mcpCreateRecipe(ctx, mcpCreateRecipeArgs{
		Name: "Pasta pesto",
		Steps: []string{
			"1. Kook de pasta.",
			"2. Mix met de pesto.",
			"Bron: https://example.com/pasta-pesto",
		},
		BaseServings:  4,
		BatchServings: &batch,
		Ingredients: []mcpIngredientArg{
			{Name: "pasta", Amount: 400, Unit: "g", GroupName: ""},
			{Name: "basilicum", Amount: 1, Unit: "pot", GroupName: "pesto"},
			{Name: "ui", Amount: 2, Unit: "", GroupName: ""},
		},
	})
	require.NoError(t, err)
	created, ok := createdMsg.(*recipesv1.CreateRecipeResponse)
	require.True(t, ok)
	assert.True(t, created.Recipe.IsDraft)
	assert.Equal(t, mcpUserID, created.Recipe.UserId)

	gotMsg, err := h.mcpGetRecipe(
		ctx,
		mcpRecipeArgs{ID: created.Recipe.Id, Servings: 0},
	)
	require.NoError(t, err)
	got, ok := gotMsg.(*recipesv1.GetRecipeResponse)
	require.True(t, ok)
	recipe := got.Recipe
	assert.True(t, recipe.IsDraft)
	assert.Equal(t, "Pasta pesto", recipe.Name)
	assert.Equal(t, int32(4), recipe.BaseServings)
	require.NotNil(t, recipe.BatchServings)
	assert.Equal(t, int32(8), *recipe.BatchServings)
	assert.Equal(
		t,
		"1. Kook de pasta.\n2. Mix met de pesto.\nBron: https://example.com/pasta-pesto",
		recipe.Instructions,
	)

	require.Len(t, recipe.Ingredients, 3)
	byName := map[string]*recipesv1.Ingredient{}
	for _, ing := range recipe.Ingredients {
		byName[ing.Name] = ing
	}
	assert.InDelta(t, 400, byName["pasta"].Amount, 0)
	assert.Equal(t, "g", byName["pasta"].Unit)
	assert.Nil(t, byName["pasta"].GroupName)
	require.NotNil(t, byName["basilicum"].GroupName)
	assert.Equal(t, "pesto", *byName["basilicum"].GroupName)
	assert.Equal(t, "pot", byName["basilicum"].Unit)
	assert.Empty(t, byName["ui"].Unit)

	listMsg, err := h.mcpListRecipes(ctx, mcpListRecipesArgs{Limit: 0, Offset: 0})
	require.NoError(t, err)
	listJSON, err := protojson.Marshal(listMsg)
	require.NoError(t, err)
	assert.Contains(t, string(listJSON), created.Recipe.Id)
}
