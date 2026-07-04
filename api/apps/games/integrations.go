package games

import (
	"context"

	"tools.xdoubleu.com/apps/games/internal/repositories"
)

// Integrations is the public-facing settings type for Backlog integrations.
type Integrations struct {
	SteamUserID string
}

func (app *Games) GetIntegrations(
	ctx context.Context,
	userID string,
) (Integrations, error) {
	i, err := app.Services.Integrations.Get(ctx, userID)
	if err != nil {
		return Integrations{}, err
	}
	return Integrations{
		SteamUserID: i.SteamUserID,
	}, nil
}

func (app *Games) SaveIntegrations(
	ctx context.Context,
	userID string,
	i Integrations,
) error {
	return app.Services.Integrations.Save(ctx, repositories.UserIntegrations{
		UserID:      userID,
		SteamUserID: i.SteamUserID,
	})
}
