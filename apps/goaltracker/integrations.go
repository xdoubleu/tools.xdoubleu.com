package goaltracker

import (
	"context"

	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
)

// Integrations is the public-facing settings type for GoalTracker integrations.
type Integrations struct {
	TodoistAPIKey    string
	TodoistProjectID string
	SteamAPIKey      string
	SteamUserID      string
	GoodreadsURL     string
}

func (app *GoalTracker) HasCompletedOnboarding(
	ctx context.Context,
	userID string,
) (bool, error) {
	return app.Services.Integrations.HasCompletedOnboarding(ctx, userID)
}

func (app *GoalTracker) GetIntegrations(
	ctx context.Context,
	userID string,
) (Integrations, error) {
	i, err := app.Services.Integrations.Get(ctx, userID)
	if err != nil {
		return Integrations{}, err
	}
	return Integrations{
		TodoistAPIKey:    i.TodoistAPIKey,
		TodoistProjectID: i.TodoistProjectID,
		SteamAPIKey:      i.SteamAPIKey,
		SteamUserID:      i.SteamUserID,
		GoodreadsURL:     i.GoodreadsURL,
	}, nil
}

func (app *GoalTracker) SaveIntegrations(
	ctx context.Context,
	userID string,
	i Integrations,
) error {
	return app.Services.Integrations.Save(ctx, repositories.UserIntegrations{
		UserID:           userID,
		TodoistAPIKey:    i.TodoistAPIKey,
		TodoistProjectID: i.TodoistProjectID,
		SteamAPIKey:      i.SteamAPIKey,
		SteamUserID:      i.SteamUserID,
		GoodreadsURL:     i.GoodreadsURL,
	})
}
