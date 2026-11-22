package games

import (
	"context"

	"tools.xdoubleu.com/apps/internal/consumption"
)

// ListConsumption returns userID's played and wishlisted Steam games as
// consumption events, for the taste app.
func (a *Games) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	return a.services.Library.ListConsumption(ctx, userID)
}
