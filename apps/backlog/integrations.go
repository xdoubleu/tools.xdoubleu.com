package backlog

import (
	"context"

	"tools.xdoubleu.com/apps/backlog/internal/repositories"
)

// Integrations is the public-facing settings type for Backlog integrations.
type Integrations struct {
	SteamAPIKey     string
	SteamUserID     string
	HardcoverAPIKey string
}

func (app *Backlog) HasCompletedOnboarding(
	ctx context.Context,
	userID string,
) (bool, error) {
	return app.Services.Integrations.HasCompletedOnboarding(ctx, userID)
}

func (app *Backlog) GetIntegrations(
	ctx context.Context,
	userID string,
) (Integrations, error) {
	i, err := app.Services.Integrations.Get(ctx, userID)
	if err != nil {
		return Integrations{}, err
	}
	return Integrations{
		SteamAPIKey:     i.SteamAPIKey,
		SteamUserID:     i.SteamUserID,
		HardcoverAPIKey: i.HardcoverAPIKey,
	}, nil
}

func (app *Backlog) SaveIntegrations(
	ctx context.Context,
	userID string,
	i Integrations,
) error {
	return app.Services.Integrations.Save(ctx, repositories.UserIntegrations{
		UserID:          userID,
		SteamAPIKey:     i.SteamAPIKey,
		SteamUserID:     i.SteamUserID,
		HardcoverAPIKey: i.HardcoverAPIKey,
	})
}
