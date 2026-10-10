package services

import (
	"context"
	"errors"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
	"tools.xdoubleu.com/internal/database"
)

// LinkKoboStoreBooks records Kobo store books and tags the library match of
// each owned one own-bol. The tag is never removed here.
func (s *BookService) LinkKoboStoreBooks(
	ctx context.Context,
	userID string,
	storeBooks []models.KoboStoreBook,
) error {
	var lib []models.UserBook
	for _, sb := range storeBooks {
		if sb.ISBN13 != nil {
			isbn := normalizeISBN(*sb.ISBN13)
			sb.ISBN13 = &isbn
			if isbn == "" {
				sb.ISBN13 = nil
			}
		}
		if err := s.books.UpsertKoboStoreBook(ctx, userID, sb); err != nil {
			return err
		}
		if sb.Owned == nil || !*sb.Owned {
			continue
		}
		ub, err := s.matchKoboStoreBook(ctx, userID, sb, &lib)
		if err != nil {
			return err
		}
		if err = s.tagOwnBol(ctx, userID, ub); err != nil {
			return err
		}
	}
	return nil
}

// RecordKoboStoreReading mirrors a store book's Kobo reading state onto its
// library match, tagging that match own-bol when the book is owned. It
// reports whether the store book is recorded; an unmatched one is a no-op.
func (s *BookService) RecordKoboStoreReading(
	ctx context.Context,
	userID string,
	reading models.KoboStoreReading,
) (bool, error) {
	sb, err := s.books.GetKoboStoreBook(ctx, userID, reading.EntitlementID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var lib []models.UserBook
	ub, err := s.matchKoboStoreBook(ctx, userID, *sb, &lib)
	if err != nil || ub == nil {
		return true, err
	}
	if *sb.Owned {
		if err = s.tagOwnBol(ctx, userID, ub); err != nil {
			return true, err
		}
	}

	percent := reading.Percent
	if reading.Finished {
		percent = models.MaxProgressPercent
	}
	if percent <= 0 {
		return true, nil
	}
	// Percent only: the store's bookmark doesn't address our files.
	//nolint:exhaustruct // see above
	state := models.BookReadingState{
		UserID:  userID,
		BookID:  ub.BookID,
		Source:  models.ReadingSourceKobo,
		Percent: percent,
		ReadAt:  reading.ReadAt,
	}
	return true, s.UpdateReadingProgress(ctx, state)
}

// matchKoboStoreBook finds the library book for a store book by ISBN-13, else
// by title and author. lib caches the library across calls; nil is no match.
func (s *BookService) matchKoboStoreBook(
	ctx context.Context,
	userID string,
	sb models.KoboStoreBook,
	lib *[]models.UserBook,
) (*models.UserBook, error) {
	if sb.ISBN13 != nil {
		ub, err := s.books.FindUserBookByISBN13(ctx, userID, *sb.ISBN13)
		if err == nil {
			return ub, nil
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, err
		}
	}
	if *lib == nil {
		l, err := s.books.GetLibrary(ctx, userID)
		if err != nil {
			return nil, err
		}
		*lib = append([]models.UserBook{}, l...)
	}
	return matchLibraryByMetadata(
		*lib,
		ebookmeta.Metadata{ //nolint:exhaustruct // title+author match
			Title:   sb.Title,
			Authors: sb.Authors,
		},
	), nil
}

// tagOwnBol adds own-bol to ub unless it's nil or already tagged.
func (s *BookService) tagOwnBol(
	ctx context.Context,
	userID string,
	ub *models.UserBook,
) error {
	if ub == nil || ub.HasTag(models.TagOwnBol) {
		return nil
	}
	if err := s.setTag(ctx, userID, ub, models.TagOwnBol, true); err != nil {
		return err
	}
	ub.Tags = append(ub.Tags, models.TagOwnBol)
	return nil
}
