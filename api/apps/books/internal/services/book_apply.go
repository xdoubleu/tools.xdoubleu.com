package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/books"
)

// ErrMismatchNotFound is returned by ApplyCSVFix when the requested
// mismatch id/difference no longer matches the current CSV+library state
// (e.g. it was already fixed, or the CSV sent back doesn't match the one the
// mismatch id was computed from).
var ErrMismatchNotFound = errors.New("mismatch not found")

// ApplyCSVFix re-runs the CSV-vs-library comparison and applies a single fix,
// identified by the mismatch id and difference tag from a prior CompareCSV
// response. The CSV is treated as the source of truth for the fixed field.
func (s *BookService) ApplyCSVFix(
	ctx context.Context,
	userID string,
	r io.Reader,
	mismatchID string,
	difference string,
) error {
	entries, err := books.ParseCSV(r)
	if err != nil {
		return err
	}

	lib, err := s.books.GetLibrary(ctx, userID)
	if err != nil {
		return err
	}

	result := CompareWithCSV(entries, lib)

	var target *CompareMismatch
	for i := range result.Mismatches {
		m := &result.Mismatches[i]
		if m.ID == mismatchID && slices.Contains(m.Differences, difference) {
			target = m
			break
		}
	}
	if target == nil {
		return ErrMismatchNotFound
	}

	switch difference {
	case DiffMissingInLibrary:
		return s.applyAddFix(ctx, userID, target)
	case "status":
		return s.applyStatusFix(ctx, userID, target)
	case "isbn":
		return s.applyISBNFix(ctx, target)
	case "title":
		return s.applyTitleFix(ctx, target)
	default:
		return fmt.Errorf("unknown difference: %q", difference)
	}
}

func (s *BookService) applyAddFix(
	ctx context.Context,
	userID string,
	target *CompareMismatch,
) error {
	if target.CSVEntry == nil {
		return ErrMismatchNotFound
	}
	ub := target.CSVEntry.UserBook
	ub.UserID = userID
	return s.books.BatchUpsert(
		ctx,
		userID,
		[]models.Book{target.CSVEntry.Book},
		[]models.UserBook{ub},
	)
}

func (s *BookService) applyStatusFix(
	ctx context.Context,
	userID string,
	target *CompareMismatch,
) error {
	if target.LibBook == nil || target.CSVEntry == nil {
		return ErrMismatchNotFound
	}
	ub := *target.LibBook
	ub.Status = target.CSVEntry.UserBook.Status
	return s.UpdateStatus(ctx, userID, ub)
}

func (s *BookService) applyISBNFix(ctx context.Context, target *CompareMismatch) error {
	if target.LibBook == nil || target.CSVEntry == nil ||
		target.CSVEntry.Book.ISBN13 == nil {
		return ErrMismatchNotFound
	}
	return s.SetBookISBN(ctx, target.LibBook.BookID, *target.CSVEntry.Book.ISBN13)
}

func (s *BookService) applyTitleFix(
	ctx context.Context,
	target *CompareMismatch,
) error {
	if target.LibBook == nil || target.CSVEntry == nil {
		return ErrMismatchNotFound
	}
	return s.SetBookTitle(ctx, target.LibBook.BookID, target.CSVEntry.Book.Title)
}
