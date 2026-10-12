package books

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/consumption"
)

// librarySearchLimit bounds the library lookup that marks a match known.
const librarySearchLimit = 20

// FindBook looks a recommended book up in the external catalogs, for the
// taste app; nil when no provider has a book of that title. Known is set when
// userID's library already holds it, by ISBN or title.
func (app *Books) FindBook(
	ctx context.Context,
	userID, title, author string,
) (*consumption.CatalogMatch, error) {
	query := strings.TrimSpace(title + " " + author)
	proposals := app.Services.Books.SearchExternal(ctx, query)
	titles := make([]string, len(proposals))
	for i, p := range proposals {
		titles[i] = p.Title
	}
	if i := consumption.BestTitle(titles, title); i >= 0 {
		p := proposals[i]
		known, err := app.inLibrary(ctx, userID, p.Title, p.ISBN13)
		if err != nil {
			return nil, err
		}
		ref := ""
		if p.ISBN13 != "" {
			ref = p.Source + ":" + p.ISBN13
		}
		return &consumption.CatalogMatch{
			Ref:          ref,
			Title:        p.Title,
			Creator:      strings.Join(p.Authors[:min(len(p.Authors), maxAuthors)], ", "),
			Year:         0,
			ImageURL:     p.CoverURL,
			URL:          "",
			Known:        known,
			OnMyServices: false,
		}, nil
	}
	return nil, nil //nolint:nilnil // no match is not an error
}

// WantBook adds the book a FindBook ref ("provider:isbn13") names to userID's
// to-read shelf, unless the library already holds it; for the taste app.
func (app *Books) WantBook(ctx context.Context, userID, ref string) error {
	provider, isbn, _ := strings.Cut(ref, ":")
	_, err := app.EnsureLibraryBook(ctx, userID, provider, isbn, models.StatusToRead)
	if errors.Is(err, ErrExternalBookNotFound) {
		return &iapp.HTTPError{
			Status: http.StatusNotFound, Message: "the book is no longer in its catalog",
		}
	}
	return err
}

// inLibrary reports whether userID's library holds isbn13 or a book named
// title.
func (app *Books) inLibrary(
	ctx context.Context,
	userID, title, isbn13 string,
) (bool, error) {
	if isbn13 != "" {
		has, err := app.Services.Books.HasISBN13(ctx, userID, isbn13)
		if err != nil || has {
			return has, err
		}
	}
	books, _, err := app.Services.Books.SearchLibrary(
		ctx, userID, title, librarySearchLimit, 0,
	)
	if err != nil {
		return false, err
	}
	for _, ub := range books {
		if ub.Book != nil && consumption.SameTitle(ub.Book.Title, title) {
			return true, nil
		}
	}
	return false, nil
}
