package backlog

import (
	"net/http"
	"slices"

	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
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

type booksProgressData struct {
	Labels    []string
	Values    []string
	DateStart string
	DateEnd   string
}

func groupByTags(userBooks []models.UserBook) []bookShelf {
	seen := map[string][]models.UserBook{}
	var order []string
	for _, ub := range userBooks {
		for _, tag := range ub.Tags {
			if models.IsSpecialTag(tag) {
				continue
			}
			if _, ok := seen[tag]; !ok {
				order = append(order, tag)
			}
			seen[tag] = append(seen[tag], ub)
		}
	}
	slices.Sort(order)
	shelves := make([]bookShelf, 0, len(order))
	for _, name := range order {
		shelves = append(shelves, bookShelf{Name: name, Books: seen[name]})
	}
	return shelves
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

func (app *Backlog) booksPageHandler(w http.ResponseWriter, _ *http.Request) error {
	tpltools.RenderWithPanic(app.Tpl, w, "books.html", nil)
	return nil
}

func (app *Backlog) booksLibraryHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	library, err := app.Services.Books.GetLibrary(r.Context(), user.ID)
	if err != nil {
		return err
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

	shelves := append(groupByStatus(library), groupByTags(library)...)
	slices.SortFunc(shelves, func(a, b bookShelf) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	tpltools.RenderWithPanic(app.Tpl, w, "books_library.html", booksPageData{
		Reading:  reading,
		Wishlist: wishlist,
		Finished: finished,
		Shelves:  shelves,
	})
	return nil
}

func (app *Backlog) booksProgressHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.BooksTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.Tpl, w, "books_progress.html", booksProgressData{
		Labels:    labels,
		Values:    values,
		DateStart: dateStart.Format(models.ProgressDateFormat),
		DateEnd:   dateEnd.Format(models.ProgressDateFormat),
	})
	return nil
}
