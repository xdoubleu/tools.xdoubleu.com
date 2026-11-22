package recipes_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func newMCPSession(t *testing.T, user sharedmodels.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			return next(context.WithValue(ctx, constants.UserContextKey, user), method, req)
		}
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// callTool calls a tool and decodes its JSON text into out.
func callTool(
	t *testing.T, session *mcp.ClientSession, name string, args map[string]any,
	out proto.Message,
) string {
	t.Helper()
	res, err := session.CallTool(
		t.Context(),
		//nolint:exhaustruct // name and arguments are all a call needs
		&mcp.CallToolParams{Name: name, Arguments: args},
	)
	require.NoError(t, err, name)
	require.False(t, res.IsError, name)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	if out != nil {
		require.NoError(t, protojson.Unmarshal([]byte(text.Text), out))
	}
	return text.Text
}

func mcpUser(id string) sharedmodels.User {
	//nolint:exhaustruct // only ID and AppAccess matter for the gate
	return sharedmodels.User{ID: id, AppAccess: []string{"recipes"}}
}

// TestMCPTools_CreateRecipeIsDraft: recipes_create_recipe persists through the
// same service layer as the Connect RPCs and always lands as a draft, even
// though its arguments carry no is_draft flag.
func TestMCPTools_CreateRecipeIsDraft(t *testing.T) {
	session := newMCPSession(t, mcpUser(userID))

	created := &recipesv1.CreateRecipeResponse{}
	callTool(t, session, "recipes_create_recipe", map[string]any{
		"name": "Pasta pesto",
		"steps": []string{
			"1. Kook de pasta.",
			"2. Mix met de pesto.",
			"Bron: https://example.com/pasta-pesto",
		},
		"base_servings":  4,
		"batch_servings": 8,
		"ingredients": []map[string]any{
			{"name": "pasta", "amount": 400, "unit": "g"},
			{"name": "basilicum", "amount": 1, "unit": "pot", "group_name": "pesto"},
			{"name": "ui", "amount": 2},
		},
	}, created)
	assert.True(t, created.Recipe.IsDraft)
	assert.Equal(t, userID, created.Recipe.UserId)

	got := &recipesv1.GetRecipeResponse{}
	callTool(
		t, session, "recipes_get_recipe", map[string]any{"id": created.Recipe.Id}, got,
	)
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

	listJSON := callTool(t, session, "recipes_list_recipes", map[string]any{}, nil)
	assert.Contains(t, listJSON, created.Recipe.Id)
	assert.NotContains(t, listJSON, "Kook de pasta",
		"the list tool leaves instructions to recipes_get_recipe")
}

// TestMCPTools_UpdateRecipe sets categories and ingredient alternatives on an
// existing recipe through recipes_update_recipe, preserving its draft state.
func TestMCPTools_UpdateRecipe(t *testing.T) {
	session := newMCPSession(t, mcpUser(userID))

	created := &recipesv1.CreateRecipeResponse{}
	callTool(t, session, "recipes_create_recipe", map[string]any{
		"name":          "Pasta of rijst",
		"steps":         []string{"1. Kook de pasta of rijst."},
		"base_servings": 2,
		"ingredients": []map[string]any{
			{"name": "pasta", "amount": 200, "unit": "g"},
			{"name": "tomaten", "amount": 3},
		},
	}, created)
	require.True(t, created.Recipe.IsDraft)

	// Publish the recipe through the service, as the user would in the app,
	// so the update path is exercised against an already-published recipe.
	_, err := setupRecipesClient(getRoutes()).UpdateRecipe(
		t.Context(), &recipesv1.UpdateRecipeRequest{
			Id: created.Recipe.Id, IsDraft: false,
			Name:         "Pasta of rijst",
			Steps:        []string{"1. Kook de pasta of rijst."},
			BaseServings: 2,
		},
	)
	require.NoError(t, err)

	// The update drops "tomaten" and adds "rijst" as an alternative to pasta:
	// UpdateRecipe is a full replacement, so the omitted ingredient must go.
	got := &recipesv1.GetRecipeResponse{}
	callTool(t, session, "recipes_update_recipe", map[string]any{
		"id":            created.Recipe.Id,
		"name":          "Pasta of rijst",
		"steps":         []string{"1. Kook de pasta of rijst."},
		"base_servings": 2,
		"meal_type":     models.MealTypeMain,
		"effort":        models.EffortLow,
		"ingredients": []map[string]any{
			{"name": "pasta", "amount": 200, "unit": "g", "alternative_group": "basis"},
			{"name": "rijst", "amount": 200, "unit": "g", "alternative_group": "basis"},
		},
	}, got)
	recipe := got.Recipe
	assert.False(t, recipe.IsDraft, "update preserves published status")
	assert.Equal(t, models.MealTypeMain, recipe.MealType)
	assert.Equal(t, models.EffortLow, recipe.Effort)
	require.Len(t, recipe.Ingredients, 2, "omitted tomaten is removed")
	byName := map[string]*recipesv1.Ingredient{}
	for _, ing := range recipe.Ingredients {
		byName[ing.Name] = ing
	}
	require.NotNil(t, byName["pasta"].AlternativeGroup)
	require.NotNil(t, byName["rijst"].AlternativeGroup)
	assert.Equal(t, "basis", *byName["pasta"].AlternativeGroup)
	assert.Equal(t, "basis", *byName["rijst"].AlternativeGroup)
}

// TestMCPTools_UpdateRecipeWithoutCaller: no authenticated user is an error.
func TestMCPTools_UpdateRecipeWithoutCaller(t *testing.T) {
	//nolint:exhaustruct // empty user: gate refuses before the tool runs
	session := newMCPSession(t, sharedmodels.User{})
	res, err := session.CallTool(
		t.Context(),
		//nolint:exhaustruct // name and arguments are all a call needs
		&mcp.CallToolParams{
			Name:      "recipes_update_recipe",
			Arguments: map[string]any{"id": "00000000-0000-0000-0000-000000000000"},
		},
	)
	if err == nil {
		assert.True(t, res.IsError)
	}
}

// TestMCPTools_WriteErrors: a failing write surfaces as a tool error.
func TestMCPTools_WriteErrors(t *testing.T) {
	session := newMCPSession(t, mcpUser(userID))
	for name, args := range map[string]map[string]any{
		"recipes_create_recipe": {
			"name": "x", "steps": []string{}, "base_servings": 1,
			"ingredients": []map[string]any{
				{"name": "y", "amount": 1e300, "unit": "g"},
			},
		},
		"recipes_update_recipe": {
			"id": "00000000-0000-0000-0000-000000000001", "name": "x",
			"steps": []string{}, "base_servings": 1,
			"ingredients": []map[string]any{}, "meal_type": "", "effort": "",
		},
	} {
		res, err := session.CallTool(
			t.Context(),
			//nolint:exhaustruct // name and arguments are all a call needs
			&mcp.CallToolParams{Name: name, Arguments: args},
		)
		require.NoError(t, err, name)
		assert.True(t, res.IsError, name)
	}
}
