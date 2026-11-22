//nolint:exhaustruct // sparse fixtures
package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToModules_ExternalBookAndDue(t *testing.T) {
	//nolint:exhaustruct //fixture sets only the fields under test
	modules := toModules([]mcpModuleArg{{Items: []mcpItemArg{
		{Description: "owned"},
		{
			Description:  "unowned",
			Due:          "every day at 20:00",
			ExternalBook: &mcpExternalBookArg{Provider: "hardcover", ProviderID: "978"},
		},
		{Description: "partial", ExternalBook: &mcpExternalBookArg{Provider: "x"}},
	}}})

	items := modules[0].Items
	assert.Nil(t, items[0].ExternalBook)
	require.NotNil(t, items[1].ExternalBook)
	assert.Equal(t, "hardcover", items[1].ExternalBook.Provider)
	assert.Equal(t, "978", items[1].ExternalBook.ProviderID)
	assert.Equal(t, "every day at 20:00", items[1].Due)
	assert.Nil(t, items[2].ExternalBook)
	assert.Equal(t, 2, items[2].SortOrder)
}

func TestToModules_LinkedBookIDAndQuiz(t *testing.T) {
	good := "6f0cc9f8-1c1b-4a3e-bf5f-1c8a0a5b0c11"
	bad := "not-a-uuid"
	//nolint:exhaustruct //fixture sets only the fields under test
	modules := toModules([]mcpModuleArg{{
		Items: []mcpItemArg{
			{Description: "a", LinkedBookID: &good},
			{Description: "b", LinkedBookID: &bad},
		},
		Quiz: []mcpQuizQuestionArg{{
			Prompt: "Q", Options: []string{"x"}, CorrectAnswerIndex: 1,
		}},
	}})

	require.NotNil(t, modules[0].Items[0].LinkedBookID)
	assert.Equal(t, good, modules[0].Items[0].LinkedBookID.String())
	assert.Nil(t, modules[0].Items[1].LinkedBookID)
	assert.Equal(t, 1, modules[0].Quiz[0].CorrectAnswerIndex)
}

func TestToResources_OrderedByPosition(t *testing.T) {
	resources := toResources([]mcpResourceArg{{Text: "a"}, {Text: "b"}})
	assert.Equal(t, "b", resources[1].Text)
	assert.Equal(t, 1, resources[1].SortOrder)
}

func TestTools_Unauthenticated(t *testing.T) {
	h := &tools{paths: nil, todoist: nil}
	ctx := context.Background()

	_, err := h.mcpListPaths(ctx, mcpListPathsArgs{})
	require.Error(t, err)
	_, err = h.mcpGetPath(ctx, mcpPathIDArgs{ID: "x"})
	require.Error(t, err)
	_, err = h.mcpGetProgress(ctx, mcpPathIDArgs{ID: "x"})
	require.Error(t, err)
	_, err = h.mcpGetTodoistStatus(ctx, mcpPathIDArgs{ID: "x"})
	require.Error(t, err)
	_, err = h.mcpCreatePath(ctx, mcpCreatePathArgs{})
	require.Error(t, err)
	_, err = h.mcpUpdatePath(ctx, mcpUpdatePathArgs{})
	require.Error(t, err)
	_, err = h.mcpRecordProgress(ctx, mcpRecordProgressArgs{})
	require.Error(t, err)
}

// TestMCPAuthoringGuide_ResourceExistsAndReads: the guide is served as an MCP
// resource, and the write-tool descriptions embed its essentials for clients
// that only read tool metadata.
func TestMCPAuthoringGuide_ResourceExistsAndReads(t *testing.T) {
	require.Contains(t, mcpCreatePathDescription, "confirm the full proposed tree")
	require.Contains(t, mcpCreatePathDescription, "modules")
	require.Contains(t, mcpUpdatePathDescription, "Wholesale-replaces")
	require.Contains(t, mcpUpdatePathDescription, "learningpaths_get_path")
	require.Contains(t, mcpRecordProgressDescription, "item_id")

	res, err := authoringGuideHandler(context.Background(),
		//nolint:exhaustruct // the handler ignores the request, so no fields needed
		&mcp.ReadResourceRequest{})
	require.NoError(t, err)
	require.Len(t, res.Contents, 1)
	c := res.Contents[0]
	assert.Equal(t, authoringGuideURI, c.URI)
	assert.Equal(t, authoringGuideMIME, c.MIMEType)
	assert.Contains(t, c.Text, "## Authoring workflow")
	assert.Contains(t, c.Text, "## Curation rubric")
	assert.Contains(t, c.Text, "learningpaths_create_path")
	assert.Contains(t, c.Text, "confirm the full proposed tree")
}

// TestMCPAuthoringGuide_RegisteredOnServer: registration doesn't panic
// (srv.AddResource panics on an invalid URI).
func TestMCPAuthoringGuide_RegisteredOnServer(t *testing.T) {
	//nolint:exhaustruct // Name/Version identify the server; nothing else matters here
	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	require.NotPanics(t, func() { registerAuthoringGuideResource(srv) })
}
