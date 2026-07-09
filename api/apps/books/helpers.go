package books

import (
	"context"
	"slices"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
)

type bookShelf struct {
	Name  string
	Books []models.UserBook
}

type booksPageData struct {
	Reading  []models.UserBook
	Wishlist []models.UserBook
	Finished []models.UserBook
	Shelves  []bookShelf
}

func groupByStatus(userBooks []models.UserBook) []bookShelf {
	standard := map[string]bool{
		models.StatusToRead:  true,
		models.StatusReading: true,
		models.StatusRead:    true,
		models.StatusDropped: true,
	}
	seen := map[string][]models.UserBook{}
	var order []string
	for _, ub := range userBooks {
		if standard[ub.Status] {
			continue
		}
		if _, ok := seen[ub.Status]; !ok {
			order = append(order, ub.Status)
		}
		seen[ub.Status] = append(seen[ub.Status], ub)
	}
	slices.Sort(order)
	shelves := make([]bookShelf, 0, len(order))
	for _, name := range order {
		shelves = append(shelves, bookShelf{Name: name, Books: seen[name]})
	}
	return shelves
}

func (app *Books) buildLibraryData(
	ctx context.Context,
	userID string,
) (booksPageData, error) {
	library, err := app.Services.Books.GetLibrary(ctx, userID)
	if err != nil {
		return booksPageData{}, err
	}

	formats, err := app.Services.Books.FormatsByUser(ctx, userID)
	if err != nil {
		return booksPageData{}, err
	}
	for i := range library {
		library[i].Formats = formats[library[i].BookID]
	}

	var reading, wishlist, finished []models.UserBook
	for _, ub := range library {
		switch ub.Status {
		case models.StatusReading:
			reading = append(reading, ub)
		case models.StatusToRead:
			wishlist = append(wishlist, ub)
		case models.StatusRead:
			finished = append(finished, ub)
		}
	}

	shelves := groupByStatus(library)
	slices.SortFunc(shelves, func(a, b bookShelf) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	return booksPageData{
		Reading:  reading,
		Wishlist: wishlist,
		Finished: finished,
		Shelves:  shelves,
	}, nil
}

func (app *Books) rebuildReadProgress(ctx context.Context, userID string) error {
	labels, values, err := app.Services.Books.BuildReadProgress(ctx, userID)
	if err != nil {
		return err
	}
	return app.Services.Progress.Save(ctx, userID, labels, values)
}

func buildFinishedAt(existing *models.UserBook, newStatus string) []time.Time {
	if newStatus != models.StatusRead {
		return nil
	}
	if existing == nil {
		return []time.Time{time.Now()}
	}
	result := append([]time.Time{}, existing.FinishedAt...)
	if existing.Status != models.StatusRead {
		result = append(result, time.Now())
	}
	return result
}

func toggleTag(tags []string, tag string, enable bool) []string {
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != tag {
			result = append(result, t)
		}
	}
	if enable {
		result = append(result, tag)
	}
	return result
}

func parseRating(raw string) *int16 {
	if raw == "" || raw == "0" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 16)
	if err != nil || n <= 0 || n > 5 {
		return nil
	}
	r16 := int16(n)
	return &r16
}

func parseDateRangeFromStrings(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if dateStart != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateStart); err == nil {
			start = t
		}
	}
	if dateEnd != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateEnd); err == nil {
			end = t
		}
	}
	return start, end
}
