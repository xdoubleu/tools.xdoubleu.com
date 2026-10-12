package podcasts

import (
	"context"
	"strconv"

	"tools.xdoubleu.com/internal/consumption"
)

// FindShow looks a recommended show up on iTunes, for the taste app; nil when
// no show has that title. Known is set when userID already favourited it.
func (a *Podcasts) FindShow(
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
		}, nil
	}
	return nil, nil //nolint:nilnil // no match is not an error
}
