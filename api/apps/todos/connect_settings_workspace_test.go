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

// TestWorkspaceScopedSettings_Lifecycle drives the workspace-scoped settings
// RPCs end to end: create a workspace, attach a label preset, section, policy
// and URL pattern to it, activate it, verify everything is returned by
// GetSettings, then mutate and remove and verify the changes stick.
func TestWorkspaceScopedSettings_Lifecycle(t *testing.T) {
	client := newSettingsClient(t)
	ctx := t.Context()

	_, err := client.AddWorkspace(
		ctx, connect.NewRequest(&todosv1.AddWorkspaceRequest{Name: "ws-lifecycle"}),
	)
	require.NoError(t, err)

	settings, err := client.GetSettings(
		ctx, connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)
	var wsID string
	for _, w := range settings.Msg.Workspaces {
		if w.Name == "ws-lifecycle" {
			wsID = w.Id
		}
	}
	require.NotEmpty(t, wsID)

	_, err = client.AddLabelPreset(ctx, connect.NewRequest(
		&todosv1.AddLabelPresetRequest{
			Category: "label", Value: "ws-lifecycle-label", WorkspaceId: wsID,
		},
	))
	require.NoError(t, err)

	_, err = client.AddSection(ctx, connect.NewRequest(
		&todosv1.AddSectionRequest{Name: "ws-lifecycle-section", WorkspaceId: wsID},
	))
	require.NoError(t, err)

	_, err = client.AddPolicy(ctx, connect.NewRequest(
		&todosv1.AddPolicyRequest{
			Text: "ws-lifecycle-policy", ReappearAfterHours: 24, WorkspaceId: wsID,
		},
	))
	require.NoError(t, err)

	_, err = client.AddURLPattern(ctx, connect.NewRequest(
		&todosv1.AddURLPatternRequest{
			UrlPrefix:    "https://ws-lifecycle.example.com",
			PlatformName: "example",
			Label:        "ex",
			WorkspaceId:  wsID,
		},
	))
	require.NoError(t, err)

	_, err = client.SetActiveWorkspace(ctx, connect.NewRequest(
		&todosv1.SetActiveWorkspaceRequest{WorkspaceId: wsID},
	))
	require.NoError(t, err)

	_, err = client.UpdateLabelColor(ctx, connect.NewRequest(
		&todosv1.UpdateLabelColorRequest{
			Category: "label", Value: "ws-lifecycle-label",
			Color: "#ff0000", WorkspaceId: wsID,
		},
	))
	require.NoError(t, err)

	settings, err = client.GetSettings(
		ctx, connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	assert.Equal(t, wsID, settings.Msg.UserSettings.ActiveWorkspaceId)

	var preset *todosv1.LabelPreset
	for _, p := range settings.Msg.LabelPresets {
		if p.Value == "ws-lifecycle-label" {
			preset = p
		}
	}
	require.NotNil(t, preset, "workspace label preset should be visible")
	assert.Equal(t, "#ff0000", preset.Color)

	var sectionInWs bool
	for _, s := range settings.Msg.Sections {
		if s.Name == "ws-lifecycle-section" && s.WorkspaceId == wsID {
			sectionInWs = true
		}
	}
	assert.True(t, sectionInWs, "section should be scoped to the workspace")

	var policyInWs bool
	for _, p := range settings.Msg.Policies {
		if p.Text == "ws-lifecycle-policy" && p.WorkspaceId == wsID {
			policyInWs = true
		}
	}
	assert.True(t, policyInWs, "policy should be scoped to the workspace")

	var patternPresent bool
	for _, u := range settings.Msg.UrlPatterns {
		if u.UrlPrefix == "https://ws-lifecycle.example.com" {
			patternPresent = true
		}
	}
	assert.True(t, patternPresent, "URL pattern should be visible")

	_, err = client.RemoveLabelPreset(ctx, connect.NewRequest(
		&todosv1.RemoveLabelPresetRequest{
			Category: "label", Value: "ws-lifecycle-label", WorkspaceId: wsID,
		},
	))
	require.NoError(t, err)

	settings, err = client.GetSettings(
		ctx, connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)
	for _, p := range settings.Msg.LabelPresets {
		assert.NotEqual(t, "ws-lifecycle-label", p.Value, "preset should be removed")
	}
}
