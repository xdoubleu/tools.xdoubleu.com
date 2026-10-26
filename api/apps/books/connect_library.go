package books

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) GetLibrary(
	ctx context.Context,
	_ *booksv1.GetLibraryRequest,
) (*booksv1.GetLibraryResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	data, err := h.app.buildLibraryData(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}
	base := h.app.clients.PublicAPIBaseURL
	return &booksv1.GetLibraryResponse{
		Library: &booksv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading, base),
			Wishlist: protoUserBooks(data.Wishlist, base),
			Finished: protoUserBooks(data.Finished, base),
			Shelves:  protoBookshelves(data.Shelves, base),
		},
	}, nil
}

func (h *booksConnectHandler) GetBooksProgress(
	ctx context.Context,
	req *booksv1.GetBooksProgressRequest,
) (*booksv1.GetBooksProgressResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	dateStart, dateEnd := parseDateRangeFromStrings(req.DateStart, req.DateEnd)
	labels, values, err := h.app.Services.Progress.GetByDates(
		ctx, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetBooksProgressResponse{
		Progress: &booksv1.BooksProgressResponse{
			Labels:    labels,
			Values:    values,
			DateStart: dateStart.Format(models.ProgressDateFormat),
			DateEnd:   dateEnd.Format(models.ProgressDateFormat),
		},
	}, nil
}

func (h *booksConnectHandler) SearchLibrary(
	ctx context.Context,
	req *booksv1.SearchLibraryRequest,
) (*booksv1.SearchLibraryResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	if req.Query == "" {
		return &booksv1.SearchLibraryResponse{
			Books: []*booksv1.UserBook{},
		}, nil
	}
	libraryResults, hasMore, err := h.app.Services.Books.SearchLibrary(
		ctx,
		user.ID,
		req.Query,
		req.Limit,
		req.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.SearchLibraryResponse{
		Books:   protoUserBooks(libraryResults, h.app.clients.PublicAPIBaseURL),
		HasMore: hasMore,
	}, nil
}

func (h *booksConnectHandler) SearchExternal(
	ctx context.Context,
	req *booksv1.SearchExternalRequest,
) (*booksv1.SearchExternalResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	if req.Query == "" {
		return &booksv1.SearchExternalResponse{
			Results: []*booksv1.ExternalBookResult{},
		}, nil
	}
	results := h.app.Services.Books.SearchExternal(ctx, req.Query)
	results = services.FilterExternalByQuery(req.Query, results)
	return &booksv1.SearchExternalResponse{
		Results: protoExternalBooks(results),
	}, nil
}

func (h *booksConnectHandler) GetExternalBook(
	ctx context.Context,
	req *booksv1.GetExternalBookRequest,
) (*booksv1.GetExternalBookResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	book, err := h.app.Services.Books.GetExternal(
		ctx,
		req.Provider,
		req.ProviderId,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetExternalBookResponse{
		Result: protoExternalBook(*book),
	}, nil
}

func (h *booksConnectHandler) CreateBook(
	ctx context.Context,
	req *booksv1.CreateBookRequest,
) (*booksv1.CreateBookResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	status := req.Status
	if status == "" {
		status = models.StatusToRead
	}
	ext := services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused here
		Source:      req.Provider,
		Title:       req.Title,
		Authors:     []string{req.Author},
		ISBN13:      req.Isbn13,
		CoverURL:    req.CoverUrl,
		Description: req.Description,
	}
	if series := seriesFromProto(
		req.SeriesName, req.SeriesPosition,
	); series != nil {
		ext.SeriesName = series.Name
		ext.SeriesPosition = series.Position
		if req.SeriesTotal > 0 {
			total := int(req.SeriesTotal)
			ext.SeriesTotal = &total
		}
	}
	initialTags := []string{}
	if req.OwnPhysical {
		initialTags = append(initialTags, models.TagOwnPhysical)
	}
	if req.OwnDigital {
		initialTags = append(initialTags, models.TagOwnDigital)
	}
	_, err := h.app.Services.Books.AddToLibrary(ctx, user.ID, ext, status, initialTags)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.CreateBookResponse{}, nil
}

func (h *booksConnectHandler) UpdateBookStatus(
	ctx context.Context,
	req *booksv1.UpdateBookStatusRequest,
) (*booksv1.UpdateBookStatusResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	existing, err := h.app.Services.Books.GetUserBook(ctx, user.ID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return nil, mapError(err)
	}
	var existingTags []string
	if existing != nil {
		existingTags = existing.Tags
	}
	existingTags = toggleTag(existingTags, models.TagFavourite, req.Favourite)
	rating := parseRating(req.Rating)
	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     req.Status,
		Tags:       existingTags,
		Rating:     rating,
		FinishedAt: buildFinishedAt(existing, req.Status),
	}
	if err = h.app.Services.Books.UpdateStatus(ctx, user.ID, ub); err != nil {
		return nil, mapError(err)
	}
	if req.Status == models.StatusRead {
		if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
			return nil, mapError(rebuildErr)
		}
	}
	return &booksv1.UpdateBookStatusResponse{}, nil
}

// UpdateFinishedAt lets a user manually correct their read-date history
// (e.g. after a resync guesses wrong, or to log a re-read). Dates are
// date-only (YYYY-MM-DD); blank entries are skipped.
func (h *booksConnectHandler) UpdateFinishedAt(
	ctx context.Context,
	req *booksv1.UpdateFinishedAtRequest,
) (*booksv1.UpdateFinishedAtResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	finishedAt := make([]time.Time, 0, len(req.FinishedAt))
	for _, raw := range req.FinishedAt {
		if raw == "" {
			continue
		}
		t, parseErr := time.Parse(time.DateOnly, raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				"invalid finished_at date",
			)
		}
		finishedAt = append(finishedAt, t)
	}
	err = h.app.Services.Books.UpdateFinishedAt(ctx, user.ID, bookID, finishedAt)
	if err != nil {
		return nil, mapError(err)
	}
	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, mapError(rebuildErr)
	}
	return &booksv1.UpdateFinishedAtResponse{}, nil
}

func (h *booksConnectHandler) RemoveBook(
	ctx context.Context,
	req *booksv1.RemoveBookRequest,
) (*booksv1.RemoveBookResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	if err = h.app.Services.Books.RemoveFromLibrary(ctx, user.ID, bookID); err != nil {
		return nil, mapError(err)
	}
	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, mapError(rebuildErr)
	}
	return &booksv1.RemoveBookResponse{}, nil
}

func (h *booksConnectHandler) UpdateProgress(
	ctx context.Context,
	req *booksv1.UpdateProgressRequest,
) (*booksv1.UpdateProgressResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	readAt := time.Now()
	if req.ReadAt != "" {
		readAt, err = time.Parse(time.RFC3339Nano, req.ReadAt)
		if err != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				"read_at must be RFC3339",
			)
		}
	}
	// An unchanged page isn't a new physical read: syncing it would move a
	// newer, finer digital position back to the page's start.
	pageChanged := req.ProgressMode == models.ProgressModePages
	if pageChanged {
		ub, getErr := h.app.Services.Books.GetUserBook(ctx, user.ID, bookID)
		pageChanged = getErr != nil || ub.CurrentPage != int(req.CurrentPage)
	}
	err = h.app.Services.Books.UpdateProgress(
		ctx,
		user.ID,
		bookID,
		req.ProgressMode,
		int(req.CurrentPage),
		int(req.ProgressPercent),
	)
	if err == nil && pageChanged {
		err = h.app.Services.Physical.SyncPage(
			ctx, user.ID, bookID, int(req.CurrentPage), readAt,
		)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.UpdateProgressResponse{}, nil
}

func (h *booksConnectHandler) GetBookContent(
	ctx context.Context,
	req *booksv1.GetBookContentRequest,
) (*booksv1.GetBookContentResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	html, err := h.app.Services.Books.GetContentHTML(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}

	if html == "" {
		// Ownership was already confirmed by GetContentHTML above, so a
		// failure here is unexpected rather than a legitimate not-found.
		ub, ubErr := h.app.Services.Books.GetUserBook(ctx, user.ID, bookID)
		if ubErr != nil {
			return nil, mapError(ubErr)
		}
		html = h.app.Services.Ingest.BackfillContentHTML(ctx, ub.Book)
	}

	return &booksv1.GetBookContentResponse{
		Html: html,
	}, nil
}
