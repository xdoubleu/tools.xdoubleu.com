package learningpaths_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestListLearningPaths_CarriesModuleAndItemCounts(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx, &learningpathsv1.CreateLearningPathRequest{
			Title: "Counted",
			Modules: []*learningpathsv1.Module{
				{Title: "M1", Items: []*learningpathsv1.Item{
					{Description: "a"}, {Description: "b"},
				}},
				{Title: "M2", Items: []*learningpathsv1.Item{{Description: "c"}}},
			},
		},
	)
	require.NoError(t, err)
	id := createResp.LearningPath.Id
	t.Cleanup(func() {
		_, _ = client.DeleteLearningPath(
			ctx, &learningpathsv1.DeleteLearningPathRequest{Id: id},
		)
	})

	_, err = testDB.Exec(context.Background(), `
		UPDATE learningpaths.items SET completed = TRUE
		WHERE description = 'a' AND module_id IN (
			SELECT id FROM learningpaths.modules WHERE learning_path_id = $1)`,
		id,
	)
	require.NoError(t, err)

	listResp, err := client.ListLearningPaths(
		ctx, &learningpathsv1.ListLearningPathsRequest{Limit: 100},
	)
	require.NoError(t, err)
	var listed *learningpathsv1.LearningPath
	for _, lp := range listResp.LearningPaths {
		if lp.Id == id {
			listed = lp
		}
	}
	require.NotNil(t, listed)
	assert.Equal(t, int32(2), listed.ModuleCount)
	assert.Equal(t, int32(3), listed.ItemCount)
	assert.Equal(t, int32(1), listed.CompletedItemCount)
}
