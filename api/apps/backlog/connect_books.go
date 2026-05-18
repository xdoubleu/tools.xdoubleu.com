package backlog

import (
	"bytes"
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

var _ backlogv1connect.BooksServiceHandler = (*booksConnectHandler)(nil)

type booksConnectHandler struct {
	app *Backlog
}

func (h *booksConnectHandler) GetSummary(
	ctx context.Context,
	_ *connect.Request[backlogv1.GetSummaryRequest],
) (*connect.Response[backlogv1.GetSummaryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	summary, err := h.app.Services.Backlog.GetSummary(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.GetSummaryResponse{
		Summary: &backlogv1.BacklogSummary{
			//nolint:gosec // safe for domain counts
			SteamCount: int32(summary.SteamCount),
			//nolint:gosec // safe for domain counts
			BooksCount: int32(summary.BooksCount),
		},
	}), nil
}

func (h *booksConnectHandler) GetUserSummary(
	ctx context.Context,
	req *connect.Request[backlogv1.GetUserSummaryRequest],
) (*connect.Response[backlogv1.GetUserSummaryResponse], error) {
	summary, err := h.app.Services.Backlog.GetSummary(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.GetUserSummaryResponse{
		Summary: &backlogv1.BacklogSummary{
			//nolint:gosec // safe for domain counts
			SteamCount: int32(summary.SteamCount),
			//nolint:gosec // safe for domain counts
			BooksCount: int32(summary.BooksCount),
		},
	}), nil
}

func (h *booksConnectHandler) GetLibrary(
	ctx context.Context,
	_ *connect.Request[backlogv1.GetLibraryRequest],
) (*connect.Response[backlogv1.GetLibraryResponse], error) {
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
	return connect.NewResponse(&backlogv1.GetLibraryResponse{
		Library: &backlogv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading),
			Wishlist: protoUserBooks(data.Wishlist),
			Finished: protoUserBooks(data.Finished),
			Shelves:  protoBookshelves(data.Shelves),
		},
	}), nil
}

func (h *booksConnectHandler) GetBooksProgress(
	ctx context.Context,
	req *connect.Request[backlogv1.GetBooksProgressRequest],
) (*connect.Response[backlogv1.GetBooksProgressResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)
	labels, values, err := h.app.Services.Progress.GetByTypeIDAndDates(
		ctx, models.BooksTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.GetBooksProgressResponse{
		Progress: &backlogv1.BooksProgressResponse{
			Labels:    labels,
			Values:    values,
			DateStart: dateStart.Format(models.ProgressDateFormat),
			DateEnd:   dateEnd.Format(models.ProgressDateFormat),
		},
	}), nil
}

func (h *booksConnectHandler) SearchLibrary(
	ctx context.Context,
	req *connect.Request[backlogv1.SearchLibraryRequest],
) (*connect.Response[backlogv1.SearchLibraryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	if req.Msg.Query == "" {
		return connect.NewResponse(&backlogv1.SearchLibraryResponse{
			Books: []*backlogv1.UserBook{},
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
	return connect.NewResponse(&backlogv1.SearchLibraryResponse{
		Books: protoUserBooks(libraryResults),
	}), nil
}

func (h *booksConnectHandler) SearchExternal(
	ctx context.Context,
	req *connect.Request[backlogv1.SearchExternalRequest],
) (*connect.Response[backlogv1.SearchExternalResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	if req.Msg.Query == "" {
		return connect.NewResponse(&backlogv1.SearchExternalResponse{
			Results: []*backlogv1.ExternalBookResult{},
		}), nil
	}
	hardcoverResults, err := h.app.Services.Books.SearchHardcover(
		ctx,
		user.ID,
		req.Msg.Query,
	)
	if err != nil {
		h.app.Logger.WarnContext(ctx, "hardcover search failed", "error", err)
	}
	return connect.NewResponse(&backlogv1.SearchExternalResponse{
		Results: protoExternalBooks(hardcoverResults),
	}), nil
}

func (h *booksConnectHandler) AddBook(
	ctx context.Context,
	req *connect.Request[backlogv1.AddBookRequest],
) (*connect.Response[backlogv1.AddBookResponse], error) {
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
	var isbn13 *string
	if req.Msg.Isbn13 != "" {
		isbn13 = &req.Msg.Isbn13
	}
	var coverURL *string
	if req.Msg.CoverUrl != "" {
		coverURL = &req.Msg.CoverUrl
	}
	var desc *string
	if req.Msg.Description != "" {
		desc = &req.Msg.Description
	}
	ext := hardcover.ExternalBook{
		Provider:    req.Msg.Provider,
		ProviderID:  req.Msg.ProviderId,
		Title:       req.Msg.Title,
		Authors:     []string{req.Msg.Author},
		ISBN13:      isbn13,
		ISBN10:      nil,
		CoverURL:    coverURL,
		Description: desc,
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
	return connect.NewResponse(&backlogv1.AddBookResponse{}), nil
}

func (h *booksConnectHandler) UpdateBookStatus(
	ctx context.Context,
	req *connect.Request[backlogv1.UpdateBookStatusRequest],
) (*connect.Response[backlogv1.UpdateBookStatusResponse], error) {
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
	var notes *string
	if req.Msg.Notes != "" {
		notes = &req.Msg.Notes
	}
	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     req.Msg.Status,
		Tags:       existingTags,
		Rating:     rating,
		Notes:      notes,
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
	return connect.NewResponse(&backlogv1.UpdateBookStatusResponse{}), nil
}

func (h *booksConnectHandler) ToggleTag(
	ctx context.Context,
	req *connect.Request[backlogv1.ToggleTagRequest],
) (*connect.Response[backlogv1.ToggleTagResponse], error) {
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
	if req.Msg.Tag == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("tag cannot be empty"),
		)
	}
	err = h.app.Services.Books.ToggleTag(ctx, user.ID, bookID, req.Msg.Tag)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.ToggleTagResponse{}), nil
}

func (h *booksConnectHandler) ImportBooks(
	ctx context.Context,
	req *connect.Request[backlogv1.ImportBooksRequest],
) (*connect.Response[backlogv1.ImportBooksResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	importCtx := context.WithoutCancel(ctx)
	reader := bytes.NewReader(req.Msg.CsvData)
	count, err := h.app.Services.Books.ImportFromCSV(importCtx, user.ID, reader)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err = h.app.rebuildReadProgress(importCtx, user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.ImportBooksResponse{
		ImportedCount: int32(count), //nolint:gosec // int32 safe for domain values
	}), nil
}

// Proto conversion helpers for books

func protoBook(book *models.Book) *backlogv1.Book {
	if book == nil {
		return nil
	}
	return &backlogv1.Book{
		Id:          book.ID.String(),
		Title:       book.Title,
		Authors:     book.Authors,
		Isbn13:      stringPtr(book.ISBN13),
		CoverUrl:    stringPtr(book.CoverURL),
		Description: stringPtr(book.Description),
	}
}

func protoUserBook(ub models.UserBook) *backlogv1.UserBook {
	finishedAt := make([]string, len(ub.FinishedAt))
	for i, t := range ub.FinishedAt {
		finishedAt[i] = t.Format(time.RFC3339)
	}
	return &backlogv1.UserBook{
		Id:         ub.ID.String(),
		UserId:     ub.UserID,
		BookId:     ub.BookID.String(),
		Book:       protoBook(ub.Book),
		Status:     ub.Status,
		Tags:       ub.Tags,
		Rating:     int32PtrFromInt16(ub.Rating),
		Notes:      stringPtr(ub.Notes),
		FinishedAt: finishedAt,
		AddedAt:    ub.AddedAt.Format(time.RFC3339),
		UpdatedAt:  ub.UpdatedAt.Format(time.RFC3339),
	}
}

func protoUserBooks(books []models.UserBook) []*backlogv1.UserBook {
	result := make([]*backlogv1.UserBook, len(books))
	for i, b := range books {
		result[i] = protoUserBook(b)
	}
	return result
}

func protoBookshelves(shelves []bookShelf) []*backlogv1.BookShelf {
	result := make([]*backlogv1.BookShelf, len(shelves))
	for i, s := range shelves {
		result[i] = &backlogv1.BookShelf{
			Name:  s.Name,
			Books: protoUserBooks(s.Books),
		}
	}
	return result
}

func protoExternalBooks(
	books []hardcover.ExternalBook,
) []*backlogv1.ExternalBookResult {
	result := make([]*backlogv1.ExternalBookResult, len(books))
	for i, b := range books {
		result[i] = &backlogv1.ExternalBookResult{
			Provider:    b.Provider,
			ProviderId:  b.ProviderID,
			Title:       b.Title,
			Authors:     b.Authors,
			Isbn13:      stringPtr(b.ISBN13),
			CoverUrl:    stringPtr(b.CoverURL),
			Description: stringPtr(b.Description),
		}
	}
	return result
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int32PtrFromInt16(i *int16) int32 {
	if i == nil {
		return 0
	}
	return int32(*i)
}
