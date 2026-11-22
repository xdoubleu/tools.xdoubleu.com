package connect

import (
	"bytes"
	"context"
	"fmt"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func (h *Handler) ImportBooks(
	ctx context.Context,
	req *booksv1.ImportBooksRequest,
) (*booksv1.ImportBooksResponse, error) {
	user := caller(ctx)
	if user == nil {
		return nil, mapError(services.ErrUnauthenticated)
	}
	count, err := h.services.Catalog.ImportBooks(
		context.WithoutCancel(ctx),
		user.ID,
		bytes.NewReader(req.CsvData),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.ImportBooksResponse{
		ImportedCount: int32(count), //nolint:gosec // int32 safe for domain values
	}, nil
}

func (h *Handler) FindDuplicates(
	ctx context.Context,
	_ *booksv1.FindDuplicatesRequest,
) (*booksv1.FindDuplicatesResponse, error) {
	groups, err := h.services.Catalog.FindDuplicates(ctx, caller(ctx))
	if err != nil {
		return nil, mapError(err)
	}

	return DuplicatesResponse(groups, h.coverBaseURL), nil
}

func (h *Handler) MergeBooks(
	ctx context.Context,
	req *booksv1.MergeBooksRequest,
) (*booksv1.MergeBooksResponse, error) {
	if err := services.RequireAdmin(caller(ctx)); err != nil {
		return nil, mapError(err)
	}

	winnerID, err := uuid.Parse(req.WinnerBookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid winner_book_id: %w", err).Error(),
		)
	}

	loserIDs := make([]uuid.UUID, 0, len(req.LoserBookIds))
	for _, raw := range req.LoserBookIds {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid loser_book_id %q: %w", raw, parseErr).Error(),
			)
		}
		loserIDs = append(loserIDs, id)
	}

	var resolvedCoverSourceBookID *uuid.UUID

	if raw := req.ResolvedCoverSourceBookId; raw != nil && *raw != "" {
		coverSourceID, parseErr := uuid.Parse(*raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf(
					"invalid resolved_cover_source_book_id: %w",
					parseErr,
				).Error(),
			)
		}

		resolvedCoverSourceBookID = &coverSourceID
	}

	deletedFiles, err := h.services.Catalog.MergeBooks(
		ctx,
		caller(ctx),
		services.MergeRequest{
			Winner:      winnerID,
			Losers:      loserIDs,
			Metadata:    protoBookToModel(req.ResolvedMetadata),
			CoverSource: resolvedCoverSourceBookID,
			Status:      req.ResolvedStatus,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.MergeBooksResponse{
		MergedGroups: 1,
		DeletedFiles: deletedFiles,
	}, nil
}

func (h *Handler) StartResync(
	ctx context.Context,
	req *booksv1.StartResyncRequest,
) (*booksv1.StartResyncResponse, error) {
	if err := h.services.Catalog.StartResync(caller(ctx), req.Force); err != nil {
		return nil, mapError(err)
	}

	return &booksv1.StartResyncResponse{}, nil
}

func (h *Handler) CancelResync(
	ctx context.Context,
	_ *booksv1.CancelResyncRequest,
) (*booksv1.CancelResyncResponse, error) {
	if err := h.services.Catalog.CancelResync(caller(ctx)); err != nil {
		return nil, mapError(err)
	}

	return &booksv1.CancelResyncResponse{}, nil
}

func (h *Handler) ListResyncProposals(
	ctx context.Context,
	_ *booksv1.ListResyncProposalsRequest,
) (*booksv1.ListResyncProposalsResponse, error) {
	proposals, err := h.services.Catalog.ListResyncProposals(ctx, caller(ctx))
	if err != nil {
		return nil, mapError(err)
	}

	return ResyncProposalsResponse(proposals), nil
}

// ParseBookID parses a book_id request field.
func ParseBookID(rawBookID string) (uuid.UUID, error) {
	bookID, err := uuid.Parse(rawBookID)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid book_id: %w", err).Error(),
		)
	}
	return bookID, nil
}

// requireAdminBookID checks admin access and parses book_id, the common
// prologue of every per-book resync RPC below.
func (h *Handler) requireAdminBookID(
	ctx context.Context,
	rawBookID string,
) (uuid.UUID, error) {
	if err := services.RequireAdmin(caller(ctx)); err != nil {
		return uuid.UUID{}, mapError(err)
	}
	return ParseBookID(rawBookID)
}

func (h *Handler) ApplyResyncChoice(
	ctx context.Context,
	req *booksv1.ApplyResyncChoiceRequest,
) (*booksv1.ApplyResyncChoiceResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	err = h.services.Catalog.ApplyResyncChoice(ctx, caller(ctx), bookID, req.Source)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.ApplyResyncChoiceResponse{}, nil
}

func (h *Handler) GetBookSources(
	ctx context.Context,
	req *booksv1.GetBookSourcesRequest,
) (*booksv1.GetBookSourcesResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	proposal, err := h.services.Catalog.GetBookSources(
		ctx,
		caller(ctx),
		bookID,
		req.GetOverrideTitle(),
		req.GetOverrideAuthor(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return BookSourcesResponse(proposal), nil
}

func (h *Handler) ApplyBookSource(
	ctx context.Context,
	req *booksv1.ApplyBookSourceRequest,
) (*booksv1.ApplyBookSourceResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	err = h.services.Catalog.ApplyBookSource(
		ctx,
		caller(ctx),
		bookID,
		req.Source,
		int(req.Index),
		req.GetOverrideTitle(),
		req.GetOverrideAuthor(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.ApplyBookSourceResponse{}, nil
}

func (h *Handler) SetBookISBN(
	ctx context.Context,
	req *booksv1.SetBookISBNRequest,
) (*booksv1.SetBookISBNResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	err = h.services.Catalog.SetBookISBN(ctx, caller(ctx), bookID, req.Isbn13)
	if err != nil {
		return nil, mapISBNError(err)
	}

	return &booksv1.SetBookISBNResponse{}, nil
}

// UpdateBook lets an admin hand-correct a catalog book's metadata directly.
func (h *Handler) UpdateBook(
	ctx context.Context,
	req *booksv1.UpdateBookRequest,
) (*booksv1.UpdateBookResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	var coverURL string
	if req.Metadata != nil {
		coverURL = req.Metadata.CoverUrl
	}
	updated, err := h.services.Catalog.UpdateBook(
		ctx, caller(ctx), bookID, protoBookToModel(req.Metadata), coverURL,
	)
	if err != nil {
		return nil, mapISBNError(err)
	}

	return &booksv1.UpdateBookResponse{
		Book: protoBook(updated, h.coverBaseURL),
	}, nil
}

func (h *Handler) GetSourceStats(
	ctx context.Context,
	_ *booksv1.GetSourceStatsRequest,
) (*booksv1.GetSourceStatsResponse, error) {
	stats, err := h.services.Catalog.GetSourceStats(ctx, caller(ctx))
	if err != nil {
		return nil, mapError(err)
	}

	return SourceStatsResponse(stats), nil
}

func (h *Handler) ListBooksInExactSources(
	ctx context.Context,
	req *booksv1.ListBooksInExactSourcesRequest,
) (*booksv1.ListBooksInExactSourcesResponse, error) {
	books, err := h.services.Catalog.ListBooksInExactSources(
		ctx, caller(ctx), req.Sources,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return BooksInExactSourcesResponse(books, h.coverBaseURL), nil
}
