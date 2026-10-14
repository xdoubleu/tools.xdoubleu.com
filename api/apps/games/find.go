package games

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/database"
)

// FindGame looks a recommended game up in the Steam store, for the taste
// app; nil when the store has no app of that name. Known is set when it is in
// userID's synced library, and otherwise Audience to its review count unless
// that lookup fails.
func (a *Games) FindGame(
	ctx context.Context,
	userID, title string,
) (*consumption.CatalogMatch, error) {
	items, err := a.Store.SearchStore(ctx, title)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	if i := consumption.BestTitle(names, title); i >= 0 {
		it := items[i]
		_, err = a.Repositories.Steam.GetGameByID(ctx, it.ID, userID)
		known := err == nil
		if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
			return nil, err
		}
		var audience *int
		if !known {
			audience = a.reviewCount(ctx, it.ID)
		}
		return &consumption.CatalogMatch{
			Ref:          strconv.Itoa(it.ID),
			Title:        it.Name,
			Creator:      "",
			Year:         0,
			ImageURL:     it.ImageURL,
			URL:          fmt.Sprintf("https://store.steampowered.com/app/%d", it.ID),
			Known:        known,
			OnMyServices: false,
			Audience:     audience,
		}, nil
	}
	return nil, nil //nolint:nilnil // no match is not an error
}

// reviewCount is appID's review count, or nil when the lookup fails, which is
// logged.
func (a *Games) reviewCount(ctx context.Context, appID int) *int {
	n, err := a.Store.ReviewCount(ctx, appID)
	if err != nil {
		a.Logger.WarnContext(ctx, "steam review count lookup failed",
			slog.Int("app_id", appID), slog.Any("error", err))
		return nil
	}
	return &n
}
