package books

import (
	"bytes"
	"context"
	"fmt"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) ImportBooks(
	ctx context.Context,
	req *booksv1.ImportBooksRequest,
) (*booksv1.ImportBooksResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	importCtx := context.WithoutCancel(ctx)
	reader := bytes.NewReader(req.CsvData)
	count, err := h.app.Services.Books.ImportFromCSV(importCtx, user.ID, reader)
	if err != nil {
		return nil, mapError(err)
	}
	if err = h.app.rebuildReadProgress(importCtx, user.ID); err != nil {
		return nil, mapError(err)
	}
	return &booksv1.ImportBooksResponse{
		ImportedCount: int32(count), //nolint:gosec // int32 safe for domain values
	}, nil
}

func (h *booksConnectHandler) FindDuplicates(
	ctx context.Context,
	_ *booksv1.FindDuplicatesRequest,
) (*booksv1.FindDuplicatesResponse, error) {
	user, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	groups, err := h.app.Services.Books.FindDuplicates(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	base := h.app.clients.PublicAPIBaseURL
	protoGroups := make([]*booksv1.DuplicateGroup, len(groups))
	for i, g := range groups {
		entries := make([]*booksv1.UserBook, len(g.Entries))
		for j, e := range g.Entries {
			entries[j] = protoUserBook(e, base)
		}
		protoGroups[i] = &booksv1.DuplicateGroup{
			Entries: entries,
			Reason:  g.Reason,
		}
	}

	return &booksv1.FindDuplicatesResponse{
		Groups: protoGroups,
	}, nil
}

func (h *booksConnectHandler) MergeBooks(
	ctx context.Context,
	req *booksv1.MergeBooksRequest,
) (*booksv1.MergeBooksResponse, error) {
	user, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
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

	deletedFiles, affectedUsers, err := h.app.Services.Books.MergeBooks(
		ctx,
		user.ID,
		winnerID,
		loserIDs,
		protoBookToModel(req.ResolvedMetadata),
		resolvedCoverSourceBookID,
		req.ResolvedStatus,
	)
	if err != nil {
		return nil, mapError(err)
	}

	for _, uid := range affectedUsers {
		if rebuildErr := h.app.rebuildReadProgress(ctx, uid); rebuildErr != nil {
			return nil, mapError(rebuildErr)
		}
	}

	return &booksv1.MergeBooksResponse{
		MergedGroups: 1,
		DeletedFiles: deletedFiles,
	}, nil
}

func (h *booksConnectHandler) StartResync(
	ctx context.Context,
	req *booksv1.StartResyncRequest,
) (*booksv1.StartResyncResponse, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.app.resyncBooksJob.Arm(req.Force)
	h.app.jobQueue.ForceRun(h.app.resyncBooksJob.ID())

	return &booksv1.StartResyncResponse{}, nil
}

func (h *booksConnectHandler) CancelResync(
	ctx context.Context,
	_ *booksv1.CancelResyncRequest,
) (*booksv1.CancelResyncResponse, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.app.resyncBooksJob.Cancel()

	return &booksv1.CancelResyncResponse{}, nil
}

func (h *booksConnectHandler) ListResyncProposals(
	ctx context.Context,
	_ *booksv1.ListResyncProposalsRequest,
) (*booksv1.ListResyncProposalsResponse, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	proposals, err := h.app.Services.Books.ListResyncProposals(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	result := make([]*booksv1.ResyncProposal, len(proposals))
	for i, p := range proposals {
		result[i] = protoResyncProposal(p)
	}

	return &booksv1.ListResyncProposalsResponse{
		Proposals: result,
	}, nil
}

// requireAdminBookID checks admin access and parses book_id — the common
// prologue shared by every per-book resync RPC below.
func (h *booksConnectHandler) requireAdminBookID(
	ctx context.Context,
	rawBookID string,
) (uuid.UUID, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return uuid.UUID{}, err
	}
	bookID, err := uuid.Parse(rawBookID)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid book_id: %w", err).Error(),
		)
	}
	return bookID, nil
}

func (h *booksConnectHandler) ApplyResyncChoice(
	ctx context.Context,
	req *booksv1.ApplyResyncChoiceRequest,
) (*booksv1.ApplyResyncChoiceResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	err = h.app.Services.Books.ApplyResyncChoice(
		ctx,
		h.app.Logger,
		bookID,
		req.Source,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.ApplyResyncChoiceResponse{}, nil
}

func (h *booksConnectHandler) GetBookSources(
	ctx context.Context,
	req *booksv1.GetBookSourcesRequest,
) (*booksv1.GetBookSourcesResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	proposal, err := h.app.Services.Books.GetBookSources(
		ctx,
		h.app.Logger,
		bookID,
		req.GetOverrideTitle(),
		req.GetOverrideAuthor(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.GetBookSourcesResponse{
		Proposal: protoResyncProposal(proposal),
	}, nil
}

func (h *booksConnectHandler) ApplyBookSource(
	ctx context.Context,
	req *booksv1.ApplyBookSourceRequest,
) (*booksv1.ApplyBookSourceResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	err = h.app.Services.Books.SyncBookSource(
		ctx,
		h.app.Logger,
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

func (h *booksConnectHandler) SetBookISBN(
	ctx context.Context,
	req *booksv1.SetBookISBNRequest,
) (*booksv1.SetBookISBNResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeAndValidateISBN13(req.Isbn13)
	if err != nil {
		return nil, err
	}

	err = h.app.Services.Books.SetBookISBN(ctx, bookID, normalized)
	if err != nil {
		return nil, mapISBNError(err)
	}

	return &booksv1.SetBookISBNResponse{}, nil
}

// UpdateBook lets an admin hand-correct a catalog book's metadata directly.
func (h *booksConnectHandler) UpdateBook(
	ctx context.Context,
	req *booksv1.UpdateBookRequest,
) (*booksv1.UpdateBookResponse, error) {
	bookID, err := h.requireAdminBookID(ctx, req.BookId)
	if err != nil {
		return nil, err
	}

	metadata := protoBookToModel(req.Metadata)
	if metadata == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"metadata is required",
		)
	}

	if metadata.ISBN13 != nil {
		normalized, isbnErr := normalizeAndValidateISBN13(*metadata.ISBN13)
		if isbnErr != nil {
			return nil, isbnErr
		}
		metadata.ISBN13 = &normalized
	}

	updated, err := h.app.Services.Books.UpdateBook(
		ctx, bookID, *metadata, req.Metadata.CoverUrl,
	)
	if err != nil {
		return nil, mapISBNError(err)
	}

	return &booksv1.UpdateBookResponse{
		Book: protoBook(updated, h.app.clients.PublicAPIBaseURL),
	}, nil
}

// Source name constants shared across the source-stats and exact-sources
// RPCs — must match repositories.sourceColumns' keys.
const (
	sourceUniCat    = "unicat"
	sourceHardcover = "hardcover"
)

func (h *booksConnectHandler) GetSourceStats(
	ctx context.Context,
	_ *booksv1.GetSourceStatsRequest,
) (*booksv1.GetSourceStatsResponse, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	stats, err := h.app.Services.Books.GetSourceStats(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	return &booksv1.GetSourceStatsResponse{
		Sources: []*booksv1.SourceStat{
			{
				Source:      sourceUniCat,
				FoundCount:  int32FromInt(stats.UniCatFound),
				UniqueCount: int32FromInt(stats.UniCatUnique),
				MissedCount: int32FromInt(stats.UniCatMissed),
			},
			{
				Source:      sourceHardcover,
				FoundCount:  int32FromInt(stats.HardcoverFound),
				UniqueCount: int32FromInt(stats.HardcoverUnique),
				MissedCount: int32FromInt(stats.HardcoverMissed),
			},
		},
		TotalBooks:       int32FromInt(stats.TotalBooks),
		NotFoundAnywhere: int32FromInt(stats.NotFoundAnywhere),
		NeverScanned:     int32FromInt(stats.NeverScanned),
		Overlaps:         sourceOverlaps(stats),
		MissedOverlaps:   sourceMissedOverlaps(stats),
	}, nil
}

// sourceOverlaps lists the "found by both sources" combo. Zero-count is kept
// — the web report filters it out.
func sourceOverlaps(stats *repositories.SourceStats) []*booksv1.SourceComboStat {
	uc, hc := sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		{Sources: []string{uc, hc}, Count: int32FromInt(stats.Both)},
	}
}

// sourceMissedOverlaps mirrors sourceOverlaps: "missed by both sources" (both
// confirmed IS FALSE).
func sourceMissedOverlaps(
	stats *repositories.SourceStats,
) []*booksv1.SourceComboStat {
	uc, hc := sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		{Sources: []string{uc, hc}, Count: int32FromInt(stats.BothMissed)},
	}
}

func (h *booksConnectHandler) ListBooksInExactSources(
	ctx context.Context,
	req *booksv1.ListBooksInExactSourcesRequest,
) (*booksv1.ListBooksInExactSourcesResponse, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	books, err := h.app.Services.Books.ListBooksInExactSources(ctx, req.Sources)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*booksv1.Book, len(books))
	for i := range books {
		out[i] = protoBook(&books[i], h.app.clients.PublicAPIBaseURL)
	}

	return &booksv1.ListBooksInExactSourcesResponse{Books: out}, nil
}
