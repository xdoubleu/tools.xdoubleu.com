package books

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) GetBookFile(
	ctx context.Context,
	req *booksv1.GetBookFileRequest,
) (*booksv1.GetBookFileResponse, error) {
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
	result, err := h.app.services.Books.GetBookFile(
		ctx, user.ID, bookID, req.Format,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.GetBookFileResponse{
		Url:       result.URL,
		ExpiresAt: result.ExpiresAt.Format(time.RFC3339),
		Format:    result.Format,
	}, nil
}

// maybeStartKEPUBConversion queues a conversion for the workers role when the
// KEPUB is missing or stale and a source exists, returning
// FileStatusConverting; otherwise kepubStatus unchanged.
func (h *booksConnectHandler) maybeStartKEPUBConversion(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	statusResult *services.KEPUBStatusResult,
) string {
	kepubStatus := statusResult.KepubStatus
	hasSource := statusResult.HasEPUB || statusResult.HasPDF
	if (kepubStatus != "" && !statusResult.KepubStale) || !hasSource {
		return kepubStatus
	}

	if err := h.app.services.KEPUBQueue.Request(ctx, userID, bookID); err != nil {
		h.app.Logger.ErrorContext(ctx, "queue kepub conversion",
			"book_id", bookID, "err", err)
		return kepubStatus
	}
	return models.FileStatusConverting
}

func (h *booksConnectHandler) RequestKEPUBConversion(
	ctx context.Context,
	req *booksv1.RequestKEPUBConversionRequest,
) (*booksv1.RequestKEPUBConversionResponse, error) {
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
	statusResult, err := h.app.services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	kepubStatus := h.maybeStartKEPUBConversion(ctx, user.ID, bookID, statusResult)
	return &booksv1.RequestKEPUBConversionResponse{
		KepubStatus: kepubStatus,
	}, nil
}

func (h *booksConnectHandler) GetKEPUBStatus(
	ctx context.Context,
	req *booksv1.GetKEPUBStatusRequest,
) (*booksv1.GetKEPUBStatusResponse, error) {
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
	result, err := h.app.services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	// A stale KEPUB is never reported ready: reconvert it, as
	// RequestKEPUBConversion would, so "converting" resolves.
	kepubStatus := result.KepubStatus
	if result.KepubStale && (result.HasEPUB || result.HasPDF) {
		kepubStatus = h.app.startKEPUBRegeneration(ctx, user.ID, bookID, kepubStatus)
	}
	return &booksv1.GetKEPUBStatusResponse{
		HasEpub:     result.HasEPUB,
		HasPdf:      result.HasPDF,
		KepubStatus: kepubStatus,
	}, nil
}

func (h *booksConnectHandler) CreateBookUpload(
	ctx context.Context,
	req *booksv1.CreateBookUploadRequest,
) (*booksv1.CreateBookUploadResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	uploadID, url, alreadyExists, err := h.app.services.Books.CreateUpload(
		ctx,
		user.ID,
		req.Filename,
		req.ContentType,
		req.Size,
		req.Checksum,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.CreateBookUploadResponse{
		UploadId:      uploadID,
		Url:           url,
		AlreadyExists: alreadyExists,
	}, nil
}

func (h *booksConnectHandler) FinalizeBookUpload(
	ctx context.Context,
	req *booksv1.FinalizeBookUploadRequest,
) (*booksv1.FinalizeBookUploadResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	// Leaving the page mustn't half-finalize an upload, but the work must still
	// fit the route's budget.
	finalizeCtx, cancel := httptools.Detach(ctx)
	defer cancel()
	result, err := h.app.services.Books.FinalizeUpload(
		finalizeCtx,
		user.ID,
		req.UploadId,
		req.Filename,
		req.ContentType,
		req.Checksum,
		req.TitleOverride,
		req.AuthorOverride,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.FinalizeBookUploadResponse{
		BookId:          result.UserBook.BookID.String(),
		FileId:          result.BookFile.ID.String(),
		RecognizedTitle: result.UserBook.Book.Title,
		MatchedExisting: result.MatchedExisting,
		Format:          result.BookFile.Format,
	}, nil
}
