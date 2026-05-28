package todos_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

// getDefaultWorkspaceID creates a workspace and returns its ID.
func getDefaultWorkspaceID(t *testing.T) string {
	t.Helper()
	client := newSettingsClient(t)
	_, err := client.AddWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.AddWorkspaceRequest{Name: "ws-for-coverage"}),
	)
	require.NoError(t, err)
	resp, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)
	for _, w := range resp.Msg.Workspaces {
		if w.Name == "ws-for-coverage" {
			return w.Id
		}
	}
	t.Fatal("workspace not found")
	return ""
}

// ── Settings invalid-ID paths ─────────────────────────────────────────────────

func TestRemoveURLPattern_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.RemoveURLPattern(
		t.Context(),
		connect.NewRequest(&todosv1.RemoveURLPatternRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRemoveSection_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.RemoveSection(
		t.Context(),
		connect.NewRequest(&todosv1.RemoveSectionRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdatePolicy_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.UpdatePolicy(
		t.Context(),
		connect.NewRequest(&todosv1.UpdatePolicyRequest{Id: "not-a-uuid", Text: "x"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRemovePolicy_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.RemovePolicy(
		t.Context(),
		connect.NewRequest(&todosv1.RemovePolicyRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteWorkspace_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.DeleteWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteWorkspaceRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// ── Settings with explicit WorkspaceId ───────────────────────────────────────

func TestAddLabelPreset_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)
	_, err := client.AddLabelPreset(
		t.Context(),
		connect.NewRequest(&todosv1.AddLabelPresetRequest{
			Category: "label", Value: "ws-id-label", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)
}

func TestRemoveLabelPreset_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)

	_, err := client.AddLabelPreset(
		t.Context(),
		connect.NewRequest(&todosv1.AddLabelPresetRequest{
			Category: "label", Value: "ws-id-remove", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)

	_, err = client.RemoveLabelPreset(
		t.Context(),
		connect.NewRequest(&todosv1.RemoveLabelPresetRequest{
			Category: "label", Value: "ws-id-remove", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)
}

func TestUpdateLabelColor_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)

	_, err := client.AddLabelPreset(
		t.Context(),
		connect.NewRequest(&todosv1.AddLabelPresetRequest{
			Category: "label", Value: "ws-id-color", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)

	_, err = client.UpdateLabelColor(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateLabelColorRequest{
			Category: "label", Value: "ws-id-color",
			Color: "#ff0000", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)
}

func TestAddSection_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)
	_, err := client.AddSection(
		t.Context(),
		connect.NewRequest(&todosv1.AddSectionRequest{
			Name: "ws-id-section", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)
}

func TestAddPolicy_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)
	_, err := client.AddPolicy(
		t.Context(),
		connect.NewRequest(&todosv1.AddPolicyRequest{
			Text:               "ws-id-policy",
			ReappearAfterHours: 24,
			WorkspaceId:        wsID,
		}),
	)
	require.NoError(t, err)
}

func TestAddURLPattern_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)
	_, err := client.AddURLPattern(
		t.Context(),
		connect.NewRequest(&todosv1.AddURLPatternRequest{
			UrlPrefix:    "https://example.com",
			PlatformName: "example",
			Label:        "ex",
			WorkspaceId:  wsID,
		}),
	)
	require.NoError(t, err)
}

func TestSetActiveWorkspace_WithID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)
	_, err := client.SetActiveWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.SetActiveWorkspaceRequest{WorkspaceId: wsID}),
	)
	require.NoError(t, err)
}
