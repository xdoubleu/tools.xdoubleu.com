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

func (h *booksConnectHandler) ClearLibrary(
	ctx context.Context,
	_ *connect.Request[booksv1.ClearLibraryRequest],
) (*connect.Response[booksv1.ClearLibraryResponse], error) {
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
	return connect.NewResponse(&booksv1.ClearLibraryResponse{
		DeletedBooks: deletedBooks,
		DeletedFiles: deletedFiles,
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

func (h *booksConnectHandler) ResyncOpenLibrary(
	ctx context.Context,
	_ *connect.Request[booksv1.ResyncOpenLibraryRequest],
) (*connect.Response[booksv1.ResyncOpenLibraryResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.app.resyncBooksJob.Arm()
	h.app.jobQueue.ForceRun(h.app.resyncBooksJob.ID())

	return connect.NewResponse(&booksv1.ResyncOpenLibraryResponse{}), nil
}

func (h *booksConnectHandler) ListCatalogBooks(
	ctx context.Context,
	_ *connect.Request[booksv1.ListCatalogBooksRequest],
) (*connect.Response[booksv1.ListCatalogBooksResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	books, err := h.app.Services.Books.ListCatalogBooks(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := make([]*booksv1.CatalogBookStatus, len(books))
	for i, b := range books {
		result[i] = protoCatalogBookStatus(b)
	}

	return connect.NewResponse(&booksv1.ListCatalogBooksResponse{
		Books: result,
	}), nil
}

func (h *booksConnectHandler) ResyncBooks(
	ctx context.Context,
	req *connect.Request[booksv1.ResyncBooksRequest],
) (*connect.Response[booksv1.ResyncBooksResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(req.Msg.BookIds))
	for _, raw := range req.Msg.BookIds {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid book_id %q: %w", raw, parseErr),
			)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("book_ids must not be empty"),
		)
	}

	h.app.resyncBooksJob.ArmFor(ids, req.Msg.Force)
	h.app.jobQueue.ForceRun(h.app.resyncBooksJob.ID())

	return connect.NewResponse(&booksv1.ResyncBooksResponse{}), nil
}

func (h *booksConnectHandler) SetBookISBN(
	ctx context.Context,
	req *connect.Request[booksv1.SetBookISBNRequest],
) (*connect.Response[booksv1.SetBookISBNResponse], error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid book_id: %w", err),
		)
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

func (h *booksConnectHandler) CompareCSV(
	ctx context.Context,
	req *connect.Request[booksv1.CompareCSVRequest],
) (*connect.Response[booksv1.CompareCSVResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	result, err := h.app.Services.Books.CompareCSV(
		ctx,
		user.ID,
		bytes.NewReader(req.Msg.CsvData),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	mismatches := make([]*booksv1.BookMismatch, len(result.Mismatches))
	for i, m := range result.Mismatches {
		mismatches[i] = &booksv1.BookMismatch{
			Csv:         protoCompareRef(m.CSV),
			Library:     protoCompareRef(m.Library),
			Differences: m.Differences,
		}
	}

	//nolint:gosec // safe for domain counts
	return connect.NewResponse(&booksv1.CompareCSVResponse{
		CsvCount:     int32(result.CSVCount),
		LibraryCount: int32(result.LibraryCount),
		MatchedCount: int32(result.MatchedCount),
		Mismatches:   mismatches,
	}), nil
}
