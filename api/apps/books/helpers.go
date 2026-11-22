package books

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/protomap"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// BuildSharedLibrary builds the reading dashboard's library payload plus the
// latest Kobo sync time ("" with no devices), for the dashboard app.
func (app *Books) BuildSharedLibrary(
	ctx context.Context,
	userID string,
) (*booksv1.LibraryResponse, string, error) {
	data, err := app.services.Books.LibraryView(ctx, userID)
	if err != nil {
		return nil, "", err
	}

	lastSeen, err := app.services.Library.KoboLastSeen(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	lastSyncedAt := ""
	if lastSeen != nil {
		lastSyncedAt = lastSeen.Format(time.RFC3339)
	}

	base := app.clients.PublicAPIBaseURL
	return &booksv1.LibraryResponse{
		Reading:  protomap.ProtoUserBooks(data.Reading, base),
		Wishlist: protomap.ProtoUserBooks(data.Wishlist, base),
		Finished: protomap.ProtoUserBooks(data.Finished, base),
		Shelves:  protomap.ProtoShelves(data.Shelves, base),
	}, lastSyncedAt, nil
}

// ErrExternalBookNotFound is returned by EnsureLibraryBook for a provider
// reference no provider resolves.
var ErrExternalBookNotFound = services.ErrExternalNotFound

// LibraryBook is one library entry as other apps see it.
type LibraryBook struct {
	Title  string
	Status string
	// ProgressPercent is the pages-aware display percent.
	ProgressPercent int
	CoverURL        string
}

// GetLibraryBookByID returns one of userID's library entries, for
// learningpaths. ErrResourceNotFound also covers another user's book, so
// existence never leaks.
func (app *Books) GetLibraryBookByID(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*LibraryBook, error) {
	ub, err := app.services.Books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	result := protomap.ProtoUserBook(*ub, app.clients.PublicAPIBaseURL)
	return &LibraryBook{
		Title:           result.Book.GetTitle(),
		Status:          result.Status,
		ProgressPercent: ub.DisplayProgressPercent(),
		CoverURL:        result.Book.GetCoverUrl(),
	}, nil
}

// EnsureLibraryBook resolves an external provider reference (as returned by
// SearchExternal) to a book in userID's library, adding it to shelf when
// absent; a book already owned keeps its shelf.
func (app *Books) EnsureLibraryBook(
	ctx context.Context,
	userID, provider, providerID, shelf string,
) (uuid.UUID, error) {
	ext, err := app.services.Books.GetExternal(ctx, provider, providerID)
	if err != nil {
		return uuid.Nil, err
	}
	ub, err := app.services.Books.AddToLibraryIfAbsent(ctx, userID, *ext, shelf)
	if err != nil {
		return uuid.Nil, err
	}
	return ub.BookID, nil
}

// BuildSharedProgress builds the reading dashboard's progress-chart payload.
func (app *Books) BuildSharedProgress(
	ctx context.Context,
	userID string,
	dateStart, dateEnd time.Time,
) (*booksv1.BooksProgressResponse, error) {
	labels, values, err := app.services.Progress.GetByDates(
		ctx,
		userID,
		dateStart,
		dateEnd,
	)
	if err != nil {
		return nil, err
	}
	return &booksv1.BooksProgressResponse{
		Labels:    labels,
		Values:    values,
		DateStart: dateStart.Format(models.ProgressDateFormat),
		DateEnd:   dateEnd.Format(models.ProgressDateFormat),
	}, nil
}
