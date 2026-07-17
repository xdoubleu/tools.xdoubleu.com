package reading

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) GetLibrary(
	ctx context.Context,
	_ *connect.Request[readingv1.GetLibraryRequest],
) (*connect.Response[readingv1.GetLibraryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	data, err := h.app.buildLibraryData(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	base := h.app.clients.PublicAPIBaseURL
	return connect.NewResponse(&readingv1.GetLibraryResponse{
		Library: &readingv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading, base),
			Wishlist: protoUserBooks(data.Wishlist, base),
			Finished: protoUserBooks(data.Finished, base),
			Shelves:  protoBookshelves(data.Shelves, base),
		},
	}), nil
}

func (h *booksConnectHandler) GetBooksProgress(
	ctx context.Context,
	req *connect.Request[readingv1.GetBooksProgressRequest],
) (*connect.Response[readingv1.GetBooksProgressResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)
	labels, values, err := h.app.Services.Progress.GetByDates(
		ctx, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.GetBooksProgressResponse{
		Progress: &readingv1.BooksProgressResponse{
			Labels:    labels,
			Values:    values,
			DateStart: dateStart.Format(models.ProgressDateFormat),
			DateEnd:   dateEnd.Format(models.ProgressDateFormat),
		},
	}), nil
}

func (h *booksConnectHandler) SearchLibrary(
	ctx context.Context,
	req *connect.Request[readingv1.SearchLibraryRequest],
) (*connect.Response[readingv1.SearchLibraryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	if req.Msg.Query == "" {
		return connect.NewResponse(&readingv1.SearchLibraryResponse{
			Books: []*readingv1.UserBook{},
		}), nil
	}
	libraryResults, err := h.app.Services.Books.SearchLibrary(
		ctx,
		user.ID,
		req.Msg.Query,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.SearchLibraryResponse{
		Books: protoUserBooks(libraryResults, h.app.clients.PublicAPIBaseURL),
	}), nil
}

func (h *booksConnectHandler) SearchExternal(
	ctx context.Context,
	req *connect.Request[readingv1.SearchExternalRequest],
) (*connect.Response[readingv1.SearchExternalResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	if req.Msg.Query == "" {
		return connect.NewResponse(&readingv1.SearchExternalResponse{
			Results: []*readingv1.ExternalBookResult{},
		}), nil
	}
	results := h.app.Services.Books.SearchExternal(ctx, req.Msg.Query)
	return connect.NewResponse(&readingv1.SearchExternalResponse{
		Results: protoExternalBooks(results),
	}), nil
}

func (h *booksConnectHandler) GetExternalBook(
	ctx context.Context,
	req *connect.Request[readingv1.GetExternalBookRequest],
) (*connect.Response[readingv1.GetExternalBookResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	book, err := h.app.Services.Books.GetExternal(
		ctx,
		req.Msg.Provider,
		req.Msg.ProviderId,
	)
	if err != nil {
		if errors.Is(err, services.ErrExternalNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.GetExternalBookResponse{
		Result: protoExternalBook(*book),
	}), nil
}

func (h *booksConnectHandler) CreateBook(
	ctx context.Context,
	req *connect.Request[readingv1.CreateBookRequest],
) (*connect.Response[readingv1.CreateBookResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	status := req.Msg.Status
	if status == "" {
		status = models.StatusToRead
	}
	ext := services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused here
		Source:      req.Msg.Provider,
		Title:       req.Msg.Title,
		Authors:     []string{req.Msg.Author},
		ISBN13:      req.Msg.Isbn13,
		CoverURL:    req.Msg.CoverUrl,
		Description: req.Msg.Description,
	}
	initialTags := []string{}
	if req.Msg.OwnPhysical {
		initialTags = append(initialTags, models.TagOwnPhysical)
	}
	if req.Msg.OwnDigital {
		initialTags = append(initialTags, models.TagOwnDigital)
	}
	_, err := h.app.Services.Books.AddToLibrary(ctx, user.ID, ext, status, initialTags)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.CreateBookResponse{}), nil
}

func (h *booksConnectHandler) UpdateBookStatus(
	ctx context.Context,
	req *connect.Request[readingv1.UpdateBookStatusRequest],
) (*connect.Response[readingv1.UpdateBookStatusResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	existing, err := h.app.Services.Books.GetUserBook(ctx, user.ID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var existingTags []string
	if existing != nil {
		existingTags = existing.Tags
	}
	existingTags = toggleTag(existingTags, models.TagFavourite, req.Msg.Favourite)
	rating := parseRating(req.Msg.Rating)
	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     req.Msg.Status,
		Tags:       existingTags,
		Rating:     rating,
		FinishedAt: buildFinishedAt(existing, req.Msg.Status),
	}
	if err = h.app.Services.Books.UpdateStatus(ctx, user.ID, ub); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if req.Msg.Status == models.StatusRead {
		if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
			return nil, connect.NewError(connect.CodeInternal, rebuildErr)
		}
	}
	return connect.NewResponse(&readingv1.UpdateBookStatusResponse{}), nil
}

// UpdateFinishedAt lets a user manually correct their read-date history
// (e.g. after a resync guesses wrong, or to log a re-read). Dates are
// date-only (YYYY-MM-DD); blank entries are skipped.
func (h *booksConnectHandler) UpdateFinishedAt(
	ctx context.Context,
	req *connect.Request[readingv1.UpdateFinishedAtRequest],
) (*connect.Response[readingv1.UpdateFinishedAtResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	finishedAt := make([]time.Time, 0, len(req.Msg.FinishedAt))
	for _, raw := range req.Msg.FinishedAt {
		if raw == "" {
			continue
		}
		t, parseErr := time.Parse(time.DateOnly, raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				errors.New("invalid finished_at date"),
			)
		}
		finishedAt = append(finishedAt, t)
	}
	err = h.app.Services.Books.UpdateFinishedAt(ctx, user.ID, bookID, finishedAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, connect.NewError(connect.CodeInternal, rebuildErr)
	}
	return connect.NewResponse(&readingv1.UpdateFinishedAtResponse{}), nil
}

func (h *booksConnectHandler) RemoveBook(
	ctx context.Context,
	req *connect.Request[readingv1.RemoveBookRequest],
) (*connect.Response[readingv1.RemoveBookResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	if err = h.app.Services.Books.RemoveFromLibrary(ctx, user.ID, bookID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, connect.NewError(connect.CodeInternal, rebuildErr)
	}
	return connect.NewResponse(&readingv1.RemoveBookResponse{}), nil
}

func (h *booksConnectHandler) UpdateProgress(
	ctx context.Context,
	req *connect.Request[readingv1.UpdateProgressRequest],
) (*connect.Response[readingv1.UpdateProgressResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	err = h.app.Services.Books.UpdateProgress(
		ctx,
		user.ID,
		bookID,
		req.Msg.ProgressMode,
		int(req.Msg.CurrentPage),
		int(req.Msg.ProgressPercent),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.UpdateProgressResponse{}), nil
}

func (h *booksConnectHandler) UpdateReadingProgress(
	ctx context.Context,
	req *connect.Request[readingv1.UpdateReadingProgressRequest],
) (*connect.Response[readingv1.UpdateReadingProgressResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	var location *string
	if req.Msg.Location != "" {
		location = &req.Msg.Location
	}
	err = h.app.Services.Books.UpdateReadingProgress(
		ctx,
		user.ID,
		bookID,
		req.Msg.Source,
		int(req.Msg.Percent),
		location,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.UpdateReadingProgressResponse{}), nil
}

func (h *booksConnectHandler) GetReadingState(
	ctx context.Context,
	req *connect.Request[readingv1.GetReadingStateRequest],
) (*connect.Response[readingv1.GetReadingStateResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	state, err := h.app.Services.Books.GetReadingState(ctx, user.ID, bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var protoState *readingv1.BookReadingStateData
	if state != nil {
		protoState = &readingv1.BookReadingStateData{
			Source:    state.Source,
			Percent:   int32FromInt(state.Percent),
			Location:  stringPtr(state.Location),
			UpdatedAt: state.UpdatedAt.Format(time.RFC3339),
		}
	}
	return connect.NewResponse(&readingv1.GetReadingStateResponse{
		State: protoState,
	}), nil
}
