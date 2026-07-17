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

func (h *booksConnectHandler) GetBookFile(
	ctx context.Context,
	req *connect.Request[readingv1.GetBookFileRequest],
) (*connect.Response[readingv1.GetBookFileResponse], error) {
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
	return connect.NewResponse(&readingv1.GetBookFileResponse{
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

func (h *booksConnectHandler) RequestKEPUBConversion(
	ctx context.Context,
	req *connect.Request[readingv1.RequestKEPUBConversionRequest],
) (*connect.Response[readingv1.RequestKEPUBConversionResponse], error) {
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
	return connect.NewResponse(&readingv1.RequestKEPUBConversionResponse{
		KepubStatus: kepubStatus,
	}), nil
}

func (h *booksConnectHandler) GetKEPUBStatus(
	ctx context.Context,
	req *connect.Request[readingv1.GetKEPUBStatusRequest],
) (*connect.Response[readingv1.GetKEPUBStatusResponse], error) {
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
	return connect.NewResponse(&readingv1.GetKEPUBStatusResponse{
		HasEpub:     result.HasEPUB,
		HasPdf:      result.HasPDF,
		KepubStatus: result.KepubStatus,
	}), nil
}

func (h *booksConnectHandler) CreateBookUpload(
	ctx context.Context,
	req *connect.Request[readingv1.CreateBookUploadRequest],
) (*connect.Response[readingv1.CreateBookUploadResponse], error) {
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
	return connect.NewResponse(&readingv1.CreateBookUploadResponse{
		UploadId:      uploadID,
		Url:           url,
		AlreadyExists: alreadyExists,
	}), nil
}

func (h *booksConnectHandler) FinalizeBookUpload(
	ctx context.Context,
	req *connect.Request[readingv1.FinalizeBookUploadRequest],
) (*connect.Response[readingv1.FinalizeBookUploadResponse], error) {
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
	return connect.NewResponse(&readingv1.FinalizeBookUploadResponse{
		BookId:          result.UserBook.BookID.String(),
		FileId:          result.BookFile.ID.String(),
		RecognizedTitle: result.UserBook.Book.Title,
		MatchedExisting: result.MatchedExisting,
		Format:          result.BookFile.Format,
	}), nil
}
