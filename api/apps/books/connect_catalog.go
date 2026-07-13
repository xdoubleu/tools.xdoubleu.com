package books

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) ImportBooks(
	ctx context.Context,
	req *connect.Request[booksv1.ImportBooksRequest],
) (*connect.Response[booksv1.ImportBooksResponse], error) {
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
	return connect.NewResponse(&booksv1.ImportBooksResponse{
		ImportedCount: int32(count), //nolint:gosec // int32 safe for domain values
	}), nil
}

func (h *booksConnectHandler) FindDuplicates(
	ctx context.Context,
	_ *connect.Request[booksv1.FindDuplicatesRequest],
) (*connect.Response[booksv1.FindDuplicatesResponse], error) {
	user, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	groups, err := h.app.Services.Books.FindDuplicates(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	return connect.NewResponse(&booksv1.FindDuplicatesResponse{
		Groups: protoGroups,
	}), nil
}

func (h *booksConnectHandler) MergeBooks(
	ctx context.Context,
	req *connect.Request[booksv1.MergeBooksRequest],
) (*connect.Response[booksv1.MergeBooksResponse], error) {
	user, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
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

	var resolvedCoverSourceBookID *uuid.UUID

	if raw := req.Msg.ResolvedCoverSourceBookId; raw != nil && *raw != "" {
		coverSourceID, parseErr := uuid.Parse(*raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf(
					"invalid resolved_cover_source_book_id: %w",
					parseErr,
				),
			)
		}

		resolvedCoverSourceBookID = &coverSourceID
	}

	deletedFiles, affectedUsers, err := h.app.Services.Books.MergeBooks(
		ctx,
		user.ID,
		winnerID,
		loserIDs,
		protoBookToModel(req.Msg.ResolvedMetadata),
		resolvedCoverSourceBookID,
		req.Msg.ResolvedStatus,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, uid := range affectedUsers {
		if rebuildErr := h.app.rebuildReadProgress(ctx, uid); rebuildErr != nil {
			return nil, connect.NewError(connect.CodeInternal, rebuildErr)
		}
	}

	return connect.NewResponse(&booksv1.MergeBooksResponse{
		MergedGroups: 1,
		DeletedFiles: deletedFiles,
	}), nil
}

func (h *booksConnectHandler) StartResync(
	ctx context.Context,
	req *connect.Request[booksv1.StartResyncRequest],
) (*connect.Response[booksv1.StartResyncResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.app.resyncBooksJob.Arm(req.Msg.Force)
	h.app.jobQueue.ForceRun(h.app.resyncBooksJob.ID())

	return connect.NewResponse(&booksv1.StartResyncResponse{}), nil
}

func (h *booksConnectHandler) CancelResync(
	ctx context.Context,
	_ *connect.Request[booksv1.CancelResyncRequest],
) (*connect.Response[booksv1.CancelResyncResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.app.resyncBooksJob.Cancel()

	return connect.NewResponse(&booksv1.CancelResyncResponse{}), nil
}

func (h *booksConnectHandler) ListResyncProposals(
	ctx context.Context,
	_ *connect.Request[booksv1.ListResyncProposalsRequest],
) (*connect.Response[booksv1.ListResyncProposalsResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	proposals, err := h.app.Services.Books.ListResyncProposals(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := make([]*booksv1.ResyncProposal, len(proposals))
	for i, p := range proposals {
		result[i] = protoResyncProposal(p)
	}

	return connect.NewResponse(&booksv1.ListResyncProposalsResponse{
		Proposals: result,
	}), nil
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
			fmt.Errorf("invalid book_id: %w", err),
		)
	}
	return bookID, nil
}

func (h *booksConnectHandler) ApplyResyncChoice(
	ctx context.Context,
	req *connect.Request[booksv1.ApplyResyncChoiceRequest],
) (*connect.Response[booksv1.ApplyResyncChoiceResponse], error) {
	bookID, err := h.requireAdminBookID(ctx, req.Msg.BookId)
	if err != nil {
		return nil, err
	}

	err = h.app.Services.Books.ApplyResyncChoice(
		ctx,
		h.app.Logger,
		bookID,
		req.Msg.Source,
	)
	if errors.Is(err, services.ErrProposalNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.ApplyResyncChoiceResponse{}), nil
}

func (h *booksConnectHandler) GetBookSources(
	ctx context.Context,
	req *connect.Request[booksv1.GetBookSourcesRequest],
) (*connect.Response[booksv1.GetBookSourcesResponse], error) {
	bookID, err := h.requireAdminBookID(ctx, req.Msg.BookId)
	if err != nil {
		return nil, err
	}

	proposal, err := h.app.Services.Books.GetBookSources(
		ctx,
		h.app.Logger,
		bookID,
		req.Msg.GetOverrideTitle(),
		req.Msg.GetOverrideAuthor(),
	)
	if errors.Is(err, services.ErrProposalNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.GetBookSourcesResponse{
		Proposal: protoResyncProposal(proposal),
	}), nil
}

func (h *booksConnectHandler) ApplyBookSource(
	ctx context.Context,
	req *connect.Request[booksv1.ApplyBookSourceRequest],
) (*connect.Response[booksv1.ApplyBookSourceResponse], error) {
	bookID, err := h.requireAdminBookID(ctx, req.Msg.BookId)
	if err != nil {
		return nil, err
	}

	err = h.app.Services.Books.SyncBookSource(
		ctx,
		h.app.Logger,
		bookID,
		req.Msg.Source,
		int(req.Msg.Index),
		req.Msg.GetOverrideTitle(),
		req.Msg.GetOverrideAuthor(),
	)
	if errors.Is(err, services.ErrProposalNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.ApplyBookSourceResponse{}), nil
}

func (h *booksConnectHandler) SetBookISBN(
	ctx context.Context,
	req *connect.Request[booksv1.SetBookISBNRequest],
) (*connect.Response[booksv1.SetBookISBNResponse], error) {
	bookID, err := h.requireAdminBookID(ctx, req.Msg.BookId)
	if err != nil {
		return nil, err
	}

	// Normalize: strip spaces, hyphens, then validate exactly 13 digits.
	normalized := strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, req.Msg.Isbn13)
	if len(normalized) != isbn13Length {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("isbn13 must be exactly 13 digits"),
		)
	}
	for _, r := range normalized {
		if r < '0' || r > '9' {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				errors.New("isbn13 must contain only digits"),
			)
		}
	}

	err = h.app.Services.Books.SetBookISBN(ctx, bookID, normalized)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("book not found"))
	}
	if errors.Is(err, database.ErrResourceConflict) {
		return nil, connect.NewError(
			connect.CodeAlreadyExists,
			errors.New("ISBN is already assigned to another book"),
		)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.SetBookISBNResponse{}), nil
}

// Source name constants shared across the source-stats and exact-sources
// RPCs — must match repositories.sourceColumns' keys.
const (
	sourceOpenLibrary = "openlibrary"
	sourceUniCat      = "unicat"
	sourceHardcover   = "hardcover"
)

func (h *booksConnectHandler) GetSourceStats(
	ctx context.Context,
	_ *connect.Request[booksv1.GetSourceStatsRequest],
) (*connect.Response[booksv1.GetSourceStatsResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	stats, err := h.app.Services.Books.GetSourceStats(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.GetSourceStatsResponse{
		Sources: []*booksv1.SourceStat{
			{
				Source:      sourceOpenLibrary,
				FoundCount:  int32FromInt(stats.OpenLibraryFound),
				UniqueCount: int32FromInt(stats.OpenLibraryUnique),
				MissedCount: int32FromInt(stats.OpenLibraryMissed),
			},
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
	}), nil
}

// sourceOverlaps lists every "found in exactly this set" combo of two or more
// sources: the 3 pairs and the all-three case. Zero-count combos are kept —
// the web report filters them out.
func sourceOverlaps(stats *repositories.SourceStats) []*booksv1.SourceComboStat {
	ol, uc, hc := sourceOpenLibrary, sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		{Sources: []string{ol, uc}, Count: int32FromInt(stats.OLUCOnly)},
		{Sources: []string{ol, hc}, Count: int32FromInt(stats.OLHCOnly)},
		{Sources: []string{uc, hc}, Count: int32FromInt(stats.UCHCOnly)},
		{Sources: []string{ol, uc, hc}, Count: int32FromInt(stats.AllThree)},
	}
}

// sourceMissedOverlaps mirrors sourceOverlaps: "missed by exactly set S"
// (every source in S confirmed IS FALSE, every source outside S confirmed IS
// TRUE) is the same book set as "found by exactly the complement of S", so each
// entry reuses a found-only count under the complementary label. All-three-
// missed is the one genuinely new number.
func sourceMissedOverlaps(
	stats *repositories.SourceStats,
) []*booksv1.SourceComboStat {
	ol, uc, hc := sourceOpenLibrary, sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		// Missed by exactly a pair → found only by the remaining source (its
		// Unique).
		{Sources: []string{ol, uc}, Count: int32FromInt(stats.HardcoverUnique)},
		{Sources: []string{ol, hc}, Count: int32FromInt(stats.UniCatUnique)},
		{Sources: []string{uc, hc}, Count: int32FromInt(stats.OpenLibraryUnique)},
		// Missed by all three.
		{
			Sources: []string{ol, uc, hc},
			Count:   int32FromInt(stats.AllThreeMissed),
		},
	}
}

func (h *booksConnectHandler) ListBooksInExactSources(
	ctx context.Context,
	req *connect.Request[booksv1.ListBooksInExactSourcesRequest],
) (*connect.Response[booksv1.ListBooksInExactSourcesResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	books, err := h.app.Services.Books.ListBooksInExactSources(ctx, req.Msg.Sources)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("sources must be 1-3 of openlibrary, unicat, hardcover"),
		)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*booksv1.Book, len(books))
	for i := range books {
		out[i] = protoBook(&books[i], h.app.clients.PublicAPIBaseURL)
	}

	return connect.NewResponse(
		&booksv1.ListBooksInExactSourcesResponse{Books: out},
	), nil
}
