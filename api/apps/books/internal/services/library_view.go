package services

import (
	"context"
	"slices"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// Shelf is a named group of library books.
type Shelf struct {
	Name  string
	Books []models.UserBook
}

// LibraryView is a user's library grouped by status.
type LibraryView struct {
	Reading  []models.UserBook
	Wishlist []models.UserBook
	Finished []models.UserBook
	Shelves  []Shelf
}

func groupByStatus(
	userBooks []models.UserBook,
	registeredShelves []string,
) []Shelf {
	// Dropped has no LibraryResponse field, so it flows through as a shelf.
	standard := map[string]bool{
		models.StatusToRead:  true,
		models.StatusReading: true,
		models.StatusRead:    true,
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
	// Registered shelves appear even when empty.
	for _, name := range registeredShelves {
		if _, ok := seen[name]; !ok {
			seen[name] = nil
			order = append(order, name)
		}
	}
	// Dropped is always shown: shelves emptied before the registry existed can't
	// be recovered from it.
	alwaysShown := []string{models.StatusDropped}
	for _, name := range alwaysShown {
		if _, ok := seen[name]; !ok {
			seen[name] = nil
			order = append(order, name)
		}
	}
	slices.Sort(order)
	shelves := make([]Shelf, 0, len(order))
	for _, name := range order {
		shelves = append(shelves, Shelf{Name: name, Books: seen[name]})
	}
	return shelves
}

// LibraryView builds userID's library grouped for display: the three standard
// shelves plus every custom one.
func (s *BookService) LibraryView(
	ctx context.Context,
	userID string,
) (LibraryView, error) {
	library, err := s.GetLibrary(ctx, userID)
	if err != nil {
		return LibraryView{}, err
	}

	files, err := s.FilesByUser(ctx, userID)
	if err != nil {
		return LibraryView{}, err
	}
	for i := range library {
		library[i].Formats = files[library[i].BookID].Formats
		library[i].FileVersions = files[library[i].BookID].Versions
		library[i].ReadSuggestion = files[library[i].BookID].ReadSuggestion()
	}

	var reading, wishlist, finished []models.UserBook
	shelfBooks := make([]models.UserBook, 0, len(library))
	for _, ub := range library {
		shelfBooks = append(shelfBooks, ub)
		switch ub.Status {
		case models.StatusReading:
			reading = append(reading, ub)
		case models.StatusToRead:
			wishlist = append(wishlist, ub)
		case models.StatusRead:
			finished = append(finished, ub)
		}
	}

	registeredShelves, err := s.ListShelves(ctx, userID)
	if err != nil {
		return LibraryView{}, err
	}

	shelves := groupByStatus(shelfBooks, registeredShelves)
	slices.SortFunc(shelves, func(a, b Shelf) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	return LibraryView{
		Reading:  reading,
		Wishlist: wishlist,
		Finished: finished,
		Shelves:  shelves,
	}, nil
}
