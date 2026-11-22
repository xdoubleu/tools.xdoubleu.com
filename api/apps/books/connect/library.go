package connect

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/protomap"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *Handler) GetLibrary(
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
	data, err := h.services.Books.LibraryView(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetLibraryResponse{
		Library: protomap.LibraryResponse(data, h.coverBaseURL),
	}, nil
}

func (h *Handler) GetBooksProgress(
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
	dateStart, dateEnd := services.ParseProgressRange(req.DateStart, req.DateEnd)
	labels, values, err := h.services.Progress.GetByDates(
		ctx, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetBooksProgressResponse{
		Progress: protomap.ProgressResponse(labels, values, dateStart, dateEnd),
	}, nil
}

func (h *Handler) SearchLibrary(
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
	libraryResults, hasMore, err := h.services.Books.SearchLibrary(
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
		Books:   protomap.ProtoUserBooks(libraryResults, h.coverBaseURL),
		HasMore: hasMore,
	}, nil
}

func (h *Handler) SearchExternal(
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
	results := h.services.Books.SearchExternal(ctx, req.Query)
	results = services.FilterExternalByQuery(req.Query, results)
	return protomap.ExternalBooksResponse(results), nil
}

func (h *Handler) GetExternalBook(
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
	book, err := h.services.Books.GetExternal(
		ctx,
		req.Provider,
		req.ProviderId,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.ExternalBookResponse(*book), nil
}

func (h *Handler) CreateBook(
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
	if series := protomap.SeriesFromProto(
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
	_, err := h.services.Books.AddToLibrary(ctx, user.ID, ext, status, initialTags)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.CreateBookResponse{}, nil
}

func (h *Handler) UpdateBookStatus(
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
	err = h.services.Library.SetStatus(ctx, user.ID, bookID, services.StatusUpdate{
		Status:    req.Status,
		Favourite: req.Favourite,
		Rating:    req.Rating,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.UpdateBookStatusResponse{}, nil
}

// UpdateFinishedAt lets a user manually correct their read-date history
// (e.g. after a resync guesses wrong, or to log a re-read). Dates are
// date-only (YYYY-MM-DD); blank entries are skipped.
func (h *Handler) UpdateFinishedAt(
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
	err = h.services.Library.SetFinishedAt(ctx, user.ID, bookID, finishedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.UpdateFinishedAtResponse{}, nil
}

func (h *Handler) RemoveBook(
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
	if err = h.services.Library.RemoveBook(ctx, user.ID, bookID); err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RemoveBookResponse{}, nil
}

func (h *Handler) UpdateProgress(
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
	err = h.services.Library.UpdateProgress(ctx, user.ID, bookID, services.ProgressUpdate{
		Mode:    req.ProgressMode,
		Page:    int(req.CurrentPage),
		Percent: int(req.ProgressPercent),
		ReadAt:  readAt,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.UpdateProgressResponse{}, nil
}

func (h *Handler) GetBookContent(
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
	html, err := h.services.Library.BookContent(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetBookContentResponse{
		Html: html,
	}, nil
}
