package services

import (
	"context"
	"errors"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
	"tools.xdoubleu.com/internal/database"
)

// LinkKoboStoreBooks records Kobo store books and marks the library match of
// each owned one (see markKoboStoreOwned). Only this sync sets own-bol, and
// it's never removed here.
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
		if err = s.markKoboStoreOwned(ctx, userID, ub); err != nil {
			return err
		}
	}
	return nil
}

// RecordKoboStoreReading mirrors a store book's Kobo reading state onto its
// library match, marking that match owned when the book is owned, and
// records the outcome on the store book. KoboStoreUnrecorded means the store
// book isn't recorded yet.
func (s *BookService) RecordKoboStoreReading(
	ctx context.Context,
	userID string,
	reading models.KoboStoreReading,
) (models.KoboStoreOutcome, error) {
	sb, err := s.books.GetKoboStoreBook(ctx, userID, reading.EntitlementID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return models.KoboStoreUnrecorded, nil
	}
	if err != nil {
		return models.KoboStoreUnrecorded, err
	}

	if reading.Finished {
		reading.Percent = models.MaxProgressPercent
	}
	outcome, err := s.mirrorKoboStoreReading(ctx, userID, *sb, reading)
	if err != nil {
		return outcome, err
	}
	return outcome, s.books.RecordKoboStoreOutcome(ctx, userID, reading, outcome)
}

func (s *BookService) mirrorKoboStoreReading(
	ctx context.Context,
	userID string,
	sb models.KoboStoreBook,
	reading models.KoboStoreReading,
) (models.KoboStoreOutcome, error) {
	var lib []models.UserBook
	ub, err := s.matchKoboStoreBook(ctx, userID, sb, &lib)
	if err != nil {
		return models.KoboStoreUnrecorded, err
	}
	if ub == nil {
		return models.KoboStoreNoMatch, nil
	}
	if *sb.Owned {
		if err = s.markKoboStoreOwned(ctx, userID, ub); err != nil {
			return models.KoboStoreUnrecorded, err
		}
	}
	if reading.Percent <= 0 {
		return models.KoboStoreNoProgress, nil
	}

	// Percent only: the store's bookmark doesn't address our files.
	//nolint:exhaustruct // see above
	state := models.BookReadingState{
		UserID:  userID,
		BookID:  ub.BookID,
		Source:  models.ReadingSourceKobo,
		Percent: reading.Percent,
		ReadAt:  reading.ReadAt,
	}
	updatedAt, err := s.upsertReadingProgress(ctx, state)
	switch {
	case err != nil:
		return models.KoboStoreUnrecorded, err
	case updatedAt == nil:
		return models.KoboStoreNotNewer, nil
	default:
		return models.KoboStoreMirrored, nil
	}
}

// ListKoboStoreBooks returns the user's recorded store books with their last
// mirror outcome and current library match.
func (s *BookService) ListKoboStoreBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboStoreBookStatus, error) {
	books, err := s.books.ListKoboStoreBooks(ctx, userID)
	if err != nil {
		return nil, err
	}
	var lib []models.UserBook
	for i := range books {
		if books[i].Match, err = s.matchKoboStoreBook(
			ctx, userID, books[i].KoboStoreBook, &lib,
		); err != nil {
			return nil, err
		}
	}
	return books, nil
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

// markKoboStoreOwned tags ub own-bol and, as a store book is digital, moves
// it to percent mode unless it has page progress or a physical copy. A nil ub
// is a no-op.
func (s *BookService) markKoboStoreOwned(
	ctx context.Context,
	userID string,
	ub *models.UserBook,
) error {
	if ub == nil {
		return nil
	}
	if !ub.HasTag(models.TagOwnBol) {
		if err := s.setTag(ctx, userID, ub, models.TagOwnBol, true); err != nil {
			return err
		}
		ub.Tags = append(ub.Tags, models.TagOwnBol)
	}
	if ub.ProgressMode != models.ProgressModePages || ub.CurrentPage > 0 ||
		ub.HasTag(models.TagOwnPhysical) {
		return nil
	}
	if err := s.books.UpdateProgress(
		ctx, userID, ub.BookID, models.ProgressModePercent, 0, ub.ProgressPercent,
	); err != nil {
		return err
	}
	ub.ProgressMode = models.ProgressModePercent
	return nil
}
