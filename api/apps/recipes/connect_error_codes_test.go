package recipes_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// otherFamilyRecipe inserts a recipe owned by another user's family.
func otherFamilyRecipe(t *testing.T) string {
	t.Helper()
	owner := uuid.NewString()
	familyID, err := familyRepo.EnsureFamily(context.Background(), owner)
	require.NoError(t, err)

	var id string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, family_id, name, base_servings)
		VALUES ($1, $2, 'Not yours', 2)
		RETURNING id::text`,
		owner, familyID,
	).Scan(&id))
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(), `DELETE FROM recipes.recipes WHERE id = $1`, id,
		)
	})
	return id
}

func TestErrorCodes(t *testing.T) {
	client := setupRecipesClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ID: userID}, //nolint:exhaustruct // only ID needed
	)
	foreign := otherFamilyRecipe(t)

	tests := []struct {
		name string
		id   string
		want connect.Code
	}{
		{"unknown recipe", uuid.NewString(), connect.CodeNotFound},
		{"invalid recipe id", "nope", connect.CodeInvalidArgument},
		{"other family's recipe", foreign, connect.CodePermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetRecipe(ctx, &recipesv1.GetRecipeRequest{Id: tt.id})
			assert.Equal(t, tt.want, connect.CodeOf(err))
		})
	}
}
