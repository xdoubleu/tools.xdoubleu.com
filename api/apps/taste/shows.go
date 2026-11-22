package taste

import (
	"context"
	"strconv"

	"tools.xdoubleu.com/apps/internal/consumption"
)

// showSource exports the favourite shows as the podcasts domain's events.
// Favourites are the only listening signal. Access to them follows taste's.
type showSource struct {
	app *Taste
}

func (showSource) GetName() string { return "taste" }

func (s showSource) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	shows, err := s.app.services.Shows.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	events := make([]consumption.Event, 0, len(shows))
	for _, sh := range shows {
		events = append(events, consumption.Event{
			Domain:  consumption.DomainPodcasts,
			ItemID:  sh.ID.String(),
			Title:   sh.Title,
			Creator: sh.Author,
			Tags:    nil,
			At:      sh.AddedAt,
			Signal:  consumption.StrongPositive,
			Reason:  "favourite",
		})
	}
	return events, nil
}

// FindShow looks a recommended show up on iTunes; nil when no show has that
// title. Known is set when userID already favourited it.
func (a *Taste) FindShow(
	ctx context.Context,
	userID, title string,
) (*consumption.CatalogMatch, error) {
	results, err := a.services.Shows.Search(ctx, userID, title)
	if err != nil {
		return nil, err
	}
	titles := make([]string, len(results))
	for i, r := range results {
		titles[i] = r.Title
	}
	if i := consumption.BestTitle(titles, title); i >= 0 {
		r := results[i]
		return &consumption.CatalogMatch{
			Ref:          strconv.FormatInt(r.ID, 10),
			Title:        r.Title,
			Creator:      r.Author,
			Year:         0,
			ImageURL:     r.ArtworkURL,
			URL:          r.AppleURL,
			Known:        r.Favourite,
			OnMyServices: false,
			Audience:     nil,
		}, nil
	}
	return nil, nil //nolint:nilnil // no match is not an error
}
