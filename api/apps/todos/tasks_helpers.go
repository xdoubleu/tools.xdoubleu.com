package todos

import (
	"context"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

type workspaceCtx struct {
	Settings   *models.UserSettings
	Workspaces []models.Workspace
}

func (a *Todos) loadWorkspaceCtx(
	ctx context.Context,
	userID string,
) (*workspaceCtx, error) {
	settings, err := a.services.Settings.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	workspaces, err := a.services.Workspaces.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &workspaceCtx{Settings: settings, Workspaces: workspaces}, nil
}
