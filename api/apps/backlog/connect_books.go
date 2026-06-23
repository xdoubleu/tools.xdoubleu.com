package backlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
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
	base := h.app.clients.PublicAPIBaseURL
	return connect.NewResponse(&backlogv1.GetLibraryResponse{
		Library: &backlogv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading, base),
			Wishlist: protoUserBooks(data.Wishlist, base),
			Finished: protoUserBooks(data.Finished, base),
			Shelves:  protoBookshelves(data.Shelves, base),
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
		Books: protoUserBooks(libraryResults, h.app.clients.PublicAPIBaseURL),
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
	results, err := h.app.Services.Books.SearchExternal(
		ctx,
		req.Msg.Query,
	)
	if err != nil {
		h.app.Logger.WarnContext(ctx, "open library search failed", "error", err)
	}
	return connect.NewResponse(&backlogv1.SearchExternalResponse{
		Results: protoExternalBooks(results),
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
	ext := openlibrary.ExternalBook{
		Provider:    req.Msg.Provider,
		ProviderID:  req.Msg.ProviderId,
		Title:       req.Msg.Title,
		Authors:     []string{req.Msg.Author},
		ISBN13:      isbn13,
		ISBN10:      nil,
		CoverURL:    coverURL,
		Description: desc,
		PageCount:   nil,
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

func (h *booksConnectHandler) UpdateProgress(
	ctx context.Context,
	req *connect.Request[backlogv1.UpdateProgressRequest],
) (*connect.Response[backlogv1.UpdateProgressResponse], error) {
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
	return connect.NewResponse(&backlogv1.UpdateProgressResponse{}), nil
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

func (h *booksConnectHandler) UpdateReadingProgress(
	ctx context.Context,
	req *connect.Request[backlogv1.UpdateReadingProgressRequest],
) (*connect.Response[backlogv1.UpdateReadingProgressResponse], error) {
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
	return connect.NewResponse(&backlogv1.UpdateReadingProgressResponse{}), nil
}

func (h *booksConnectHandler) GetReadingState(
	ctx context.Context,
	req *connect.Request[backlogv1.GetReadingStateRequest],
) (*connect.Response[backlogv1.GetReadingStateResponse], error) {
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
	var protoState *backlogv1.BookReadingStateData
	if state != nil {
		protoState = &backlogv1.BookReadingStateData{
			Source:    state.Source,
			Percent:   int32FromInt(state.Percent),
			Location:  stringPtr(state.Location),
			UpdatedAt: state.UpdatedAt.Format(time.RFC3339),
		}
	}
	return connect.NewResponse(&backlogv1.GetReadingStateResponse{
		State: protoState,
	}), nil
}

func (h *booksConnectHandler) GetBookFile(
	ctx context.Context,
	req *connect.Request[backlogv1.GetBookFileRequest],
) (*connect.Response[backlogv1.GetBookFileResponse], error) {
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
	result, err := h.app.Services.Books.GetBookFile(
		ctx, user.ID, bookID, req.Msg.Format,
	)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.GetBookFileResponse{
		Url:       result.URL,
		ExpiresAt: result.ExpiresAt.Format(time.RFC3339),
		Format:    result.Format,
	}), nil
}

// maybeStartKEPUBConversion checks whether a background KEPUB conversion should
// be started for the given book and user. If no KEPUB row exists yet and a
// convertible source (EPUB or PDF) is available it launches EnsureKEPUB in a
// detached goroutine and returns models.FileStatusConverting; otherwise it
// returns the current kepubStatus unchanged.
//
// whenKEPUBOnly controls the wantsKEPUB gate: pass true to respect the user's
// raw-PDF preference (EnableKoboSync), false to always convert regardless
// (RequestKEPUBConversion for in-browser preview).
func (h *booksConnectHandler) maybeStartKEPUBConversion(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	statusResult *services.KEPUBStatusResult,
	whenKEPUBOnly bool,
) (string, error) {
	kepubStatus := statusResult.KepubStatus
	hasSource := statusResult.HasEPUB || statusResult.HasPDF
	if kepubStatus != "" || !hasSource {
		return kepubStatus, nil
	}

	if whenKEPUBOnly {
		koboFormat, err := h.app.Services.Books.GetKoboFileFormat(ctx, userID, bookID)
		if err != nil {
			return "", err
		}
		if koboFormat != models.FileFormatKEPUB {
			return kepubStatus, nil
		}
	}

	convCtx := context.WithoutCancel(ctx)
	go func() {
		_, _ = h.app.Services.Conversion.EnsureKEPUB(convCtx, userID, bookID)
	}()
	return models.FileStatusConverting, nil
}

func (h *booksConnectHandler) EnableKoboSync(
	ctx context.Context,
	req *connect.Request[backlogv1.EnableKoboSyncRequest],
) (*connect.Response[backlogv1.EnableKoboSyncResponse], error) {
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
	if err = h.app.Services.Books.EnableKoboSync(ctx, user.ID, bookID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	statusResult, err := h.app.Services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	kepubStatus, convErr := h.maybeStartKEPUBConversion(
		ctx, user.ID, bookID, statusResult, true,
	)
	if convErr != nil {
		return nil, connect.NewError(connect.CodeInternal, convErr)
	}
	return connect.NewResponse(&backlogv1.EnableKoboSyncResponse{
		KepubStatus: kepubStatus,
	}), nil
}

func (h *booksConnectHandler) RequestKEPUBConversion(
	ctx context.Context,
	req *connect.Request[backlogv1.RequestKEPUBConversionRequest],
) (*connect.Response[backlogv1.RequestKEPUBConversionResponse], error) {
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
	statusResult, err := h.app.Services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// whenKEPUBOnly=false: always convert regardless of the kobo-format-pdf tag,
	// so the user can preview the EPUB output before deciding on a Kobo sync format.
	kepubStatus, convErr := h.maybeStartKEPUBConversion(
		ctx, user.ID, bookID, statusResult, false,
	)
	if convErr != nil {
		return nil, connect.NewError(connect.CodeInternal, convErr)
	}
	return connect.NewResponse(&backlogv1.RequestKEPUBConversionResponse{
		KepubStatus: kepubStatus,
	}), nil
}

func (h *booksConnectHandler) GetKEPUBStatus(
	ctx context.Context,
	req *connect.Request[backlogv1.GetKEPUBStatusRequest],
) (*connect.Response[backlogv1.GetKEPUBStatusResponse], error) {
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
	result, err := h.app.Services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.GetKEPUBStatusResponse{
		HasEpub:     result.HasEPUB,
		HasPdf:      result.HasPDF,
		KepubStatus: result.KepubStatus,
	}), nil
}

func koboDeviceProto(d models.KoboDevice) *backlogv1.KoboDevice {
	lastSeen := ""
	if d.LastSeenAt != nil {
		lastSeen = d.LastSeenAt.Format(time.RFC3339)
	}
	return &backlogv1.KoboDevice{
		Id:         d.ID,
		Name:       d.Name,
		Serial:     d.Serial,
		CreatedAt:  d.CreatedAt.Format(time.RFC3339),
		LastSeenAt: lastSeen,
	}
}

func (h *booksConnectHandler) RegisterKoboDevice(
	ctx context.Context,
	req *connect.Request[backlogv1.RegisterKoboDeviceRequest],
) (*connect.Response[backlogv1.RegisterKoboDeviceResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	device, rawToken, err := h.app.Services.Integrations.RegisterKoboDevice(
		ctx, user.ID, req.Msg.Name, req.Msg.Serial,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.RegisterKoboDeviceResponse{
		Device:   koboDeviceProto(device),
		RawToken: rawToken,
	}), nil
}

func (h *booksConnectHandler) ListKoboDevices(
	ctx context.Context,
	_ *connect.Request[backlogv1.ListKoboDevicesRequest],
) (*connect.Response[backlogv1.ListKoboDevicesResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	devices, err := h.app.Services.Integrations.ListKoboDevices(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &backlogv1.ListKoboDevicesResponse{
		Devices: make([]*backlogv1.KoboDevice, len(devices)),
	}
	for i, d := range devices {
		resp.Devices[i] = koboDeviceProto(d)
	}
	return connect.NewResponse(resp), nil
}

func (h *booksConnectHandler) DisconnectKoboDevice(
	ctx context.Context,
	req *connect.Request[backlogv1.DisconnectKoboDeviceRequest],
) (*connect.Response[backlogv1.DisconnectKoboDeviceResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	deviceID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid device ID"),
		)
	}
	err = h.app.Services.Integrations.DisconnectKoboDevice(ctx, user.ID, deviceID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil, connect.NewError(
				connect.CodeNotFound,
				errors.New("device not found"),
			)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backlogv1.DisconnectKoboDeviceResponse{}), nil
}

func (h *booksConnectHandler) CreateBookUpload(
	ctx context.Context,
	req *connect.Request[backlogv1.CreateBookUploadRequest],
) (*connect.Response[backlogv1.CreateBookUploadResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	uploadID, url, alreadyExists, err := h.app.Services.Books.CreateUpload(
		ctx,
		user.ID,
		req.Msg.Filename,
		req.Msg.ContentType,
		req.Msg.Size,
		req.Msg.Checksum,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrFileTooLarge):
			return nil, connect.NewError(connect.CodeResourceExhausted, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&backlogv1.CreateBookUploadResponse{
		UploadId:      uploadID,
		Url:           url,
		AlreadyExists: alreadyExists,
	}), nil
}

func (h *booksConnectHandler) FinalizeBookUpload(
	ctx context.Context,
	req *connect.Request[backlogv1.FinalizeBookUploadRequest],
) (*connect.Response[backlogv1.FinalizeBookUploadResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	result, err := h.app.Services.Books.FinalizeUpload(
		context.WithoutCancel(ctx),
		user.ID,
		req.Msg.UploadId,
		req.Msg.Filename,
		req.Msg.ContentType,
		req.Msg.Checksum,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidFormat):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, services.ErrUnrecognizedBook):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, services.ErrInvalidUploadID):
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		case errors.Is(err, services.ErrUploadMissing):
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&backlogv1.FinalizeBookUploadResponse{
		BookId:          result.UserBook.BookID.String(),
		FileId:          result.BookFile.ID.String(),
		RecognizedTitle: result.UserBook.Book.Title,
		MatchedExisting: result.MatchedExisting,
		Format:          result.BookFile.Format,
	}), nil
}

// Proto conversion helpers for books

// coverProxyURL returns the proxy URL for a book cover served through our own
// endpoint. When coverBaseURL is empty (e.g. in tests) it returns an empty
// string, and the book will have no cover_url in the proto response.
func coverProxyURL(bookID fmt.Stringer, coverBaseURL string) string {
	if coverBaseURL == "" {
		return ""
	}

	return coverBaseURL + "/backlog/api/cover/" + bookID.String()
}

func protoBook(book *models.Book, coverBaseURL string) *backlogv1.Book {
	if book == nil {
		return nil
	}

	// Only expose a cover URL when the book has one stored. The actual image is
	// served through our proxy endpoint (which caches it in R2) rather than
	// directly from Open Library.
	proxyURL := ""
	if book.CoverURL != nil && *book.CoverURL != "" {
		proxyURL = coverProxyURL(book.ID, coverBaseURL)
	}

	return &backlogv1.Book{
		Id:          book.ID.String(),
		Title:       book.Title,
		Authors:     book.Authors,
		Isbn13:      stringPtr(book.ISBN13),
		CoverUrl:    proxyURL,
		Description: stringPtr(book.Description),
		PageCount:   int32FromIntPtr(book.PageCount),
	}
}

func protoUserBook(ub models.UserBook, coverBaseURL string) *backlogv1.UserBook {
	finishedAt := make([]string, len(ub.FinishedAt))
	for i, t := range ub.FinishedAt {
		finishedAt[i] = t.Format(time.RFC3339)
	}

	return &backlogv1.UserBook{
		Id:              ub.ID.String(),
		UserId:          ub.UserID,
		BookId:          ub.BookID.String(),
		Book:            protoBook(ub.Book, coverBaseURL),
		Status:          ub.Status,
		Tags:            ub.Tags,
		Formats:         ub.Formats,
		Rating:          int32PtrFromInt16(ub.Rating),
		Notes:           stringPtr(ub.Notes),
		FinishedAt:      finishedAt,
		ProgressMode:    ub.ProgressMode,
		CurrentPage:     int32FromInt(ub.CurrentPage),
		ProgressPercent: int32FromInt(ub.ProgressPercent),
		AddedAt:         ub.AddedAt.Format(time.RFC3339),
		UpdatedAt:       ub.UpdatedAt.Format(time.RFC3339),
	}
}

func protoUserBooks(
	books []models.UserBook,
	coverBaseURL string,
) []*backlogv1.UserBook {
	result := make([]*backlogv1.UserBook, len(books))
	for i, b := range books {
		result[i] = protoUserBook(b, coverBaseURL)
	}

	return result
}

func protoBookshelves(shelves []bookShelf, coverBaseURL string) []*backlogv1.BookShelf {
	result := make([]*backlogv1.BookShelf, len(shelves))
	for i, s := range shelves {
		result[i] = &backlogv1.BookShelf{
			Name:  s.Name,
			Books: protoUserBooks(s.Books, coverBaseURL),
		}
	}
	return result
}

func protoExternalBooks(
	books []openlibrary.ExternalBook,
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

func (h *booksConnectHandler) ClearLibrary(
	ctx context.Context,
	_ *connect.Request[backlogv1.ClearLibraryRequest],
) (*connect.Response[backlogv1.ClearLibraryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	deletedBooks, deletedFiles, err := h.app.Services.Books.ClearLibrary(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, connect.NewError(connect.CodeInternal, rebuildErr)
	}
	return connect.NewResponse(&backlogv1.ClearLibraryResponse{
		DeletedBooks: deletedBooks,
		DeletedFiles: deletedFiles,
	}), nil
}

func (h *booksConnectHandler) FindDuplicates(
	ctx context.Context,
	_ *connect.Request[backlogv1.FindDuplicatesRequest],
) (*connect.Response[backlogv1.FindDuplicatesResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	groups, err := h.app.Services.Books.FindDuplicates(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	base := h.app.clients.PublicAPIBaseURL
	protoGroups := make([]*backlogv1.DuplicateGroup, len(groups))
	for i, g := range groups {
		entries := make([]*backlogv1.UserBook, len(g.Entries))
		for j, e := range g.Entries {
			entries[j] = protoUserBook(e, base)
		}
		protoGroups[i] = &backlogv1.DuplicateGroup{
			Entries: entries,
			Reason:  g.Reason,
		}
	}

	return connect.NewResponse(&backlogv1.FindDuplicatesResponse{
		Groups: protoGroups,
	}), nil
}

func (h *booksConnectHandler) MergeBooks(
	ctx context.Context,
	req *connect.Request[backlogv1.MergeBooksRequest],
) (*connect.Response[backlogv1.MergeBooksResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	winnerID, err := uuid.Parse(req.Msg.WinnerBookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid winner_book_id: %w", err),
		)
	}

	loserIDs := make([]uuid.UUID, 0, len(req.Msg.LoserBookIds))
	for _, raw := range req.Msg.LoserBookIds {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid loser_book_id %q: %w", raw, parseErr),
			)
		}
		loserIDs = append(loserIDs, id)
	}

	deletedFiles, err := h.app.Services.Books.MergeBooks(
		ctx,
		user.ID,
		winnerID,
		loserIDs,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if rebuildErr := h.app.rebuildReadProgress(ctx, user.ID); rebuildErr != nil {
		return nil, connect.NewError(connect.CodeInternal, rebuildErr)
	}

	return connect.NewResponse(&backlogv1.MergeBooksResponse{
		MergedGroups: 1,
		DeletedFiles: deletedFiles,
	}), nil
}

func (h *booksConnectHandler) ResyncOpenLibrary(
	ctx context.Context,
	_ *connect.Request[backlogv1.ResyncOpenLibraryRequest],
) (*connect.Response[backlogv1.ResyncOpenLibraryResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	h.app.resyncBooksJob.Arm()
	h.app.jobQueue.ForceRun(h.app.resyncBooksJob.ID())

	return connect.NewResponse(&backlogv1.ResyncOpenLibraryResponse{}), nil
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

func int32FromInt(i int) int32 {
	//nolint:gosec // safe for domain page/percent values
	return int32(i)
}

func int32FromIntPtr(i *int) int32 {
	if i == nil {
		return 0
	}
	return int32FromInt(*i)
}
