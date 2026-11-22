package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/protomap"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/communication/httptools"
)

func (h *Handler) GetBookFile(
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
	result, err := h.services.Books.GetBookFile(
		ctx, user.ID, bookID, req.Format,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.BookFileResponse(result), nil
}

func (h *Handler) RequestKEPUBConversion(
	ctx context.Context,
	req *booksv1.RequestKEPUBConversionRequest,
) (*booksv1.RequestKEPUBConversionResponse, error) {
	user := caller(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "unauthorized")
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid book ID")
	}
	status, err := h.services.Library.RequestKEPUB(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RequestKEPUBConversionResponse{KepubStatus: status}, nil
}

func (h *Handler) GetKEPUBStatus(
	ctx context.Context,
	req *booksv1.GetKEPUBStatusRequest,
) (*booksv1.GetKEPUBStatusResponse, error) {
	user := caller(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "unauthorized")
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid book ID")
	}
	view, err := h.services.Library.KEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.KEPUBStatusResponse(view), nil
}

func (h *Handler) CreateBookUpload(
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
	uploadID, url, alreadyExists, err := h.services.Books.CreateUpload(
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

func (h *Handler) FinalizeBookUpload(
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
	result, err := h.services.Books.FinalizeUpload(
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
