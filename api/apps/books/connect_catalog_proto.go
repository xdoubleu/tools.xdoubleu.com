package books

import (
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

const isbn13Length = 13

// protoCatalogBookStatus maps a catalog book model to the admin-facing proto
// view used by ListCatalogBooks / SelectiveResync.
func protoCatalogBookStatus(b models.Book) *booksv1.CatalogBookStatus {
	const (
		statusFound    = "found"
		statusNotFound = "not_found"
	)

	olStatus := ""
	if b.OpenLibraryFound != nil {
		if *b.OpenLibraryFound {
			olStatus = statusFound
		} else {
			olStatus = statusNotFound
		}
	}

	gbStatus := ""
	if b.GoogleBooksFound != nil {
		if *b.GoogleBooksFound {
			gbStatus = statusFound
		} else {
			gbStatus = statusNotFound
		}
	}

	ucStatus := ""
	if b.UniCatFound != nil {
		if *b.UniCatFound {
			ucStatus = statusFound
		} else {
			ucStatus = statusNotFound
		}
	}

	lastResyncAt := ""
	if b.LastResyncAt != nil {
		lastResyncAt = b.LastResyncAt.UTC().Format(time.RFC3339)
	}

	return &booksv1.CatalogBookStatus{
		Id:                b.ID.String(),
		Title:             b.Title,
		Authors:           b.Authors,
		Isbn13:            stringPtr(b.ISBN13),
		HasCover:          b.CoverURL != nil && *b.CoverURL != "",
		HasDescription:    b.Description != nil && *b.Description != "",
		HasPageCount:      b.PageCount != nil,
		OpenlibraryStatus: olStatus,
		GooglebooksStatus: gbStatus,
		UnicatStatus:      ucStatus,
		LastResyncAt:      lastResyncAt,
	}
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// protoBookToModel converts a proto Book message to a models.Book, carrying
// only the catalog metadata fields. The ID field is intentionally left as the
// zero UUID; callers must set it to the correct book ID before persisting.
func protoBookToModel(pb *booksv1.Book) *models.Book {
	if pb == nil {
		return nil
	}

	m := &models.Book{ //nolint:exhaustruct // only catalog fields; ID set by caller
		Title:   pb.Title,
		Authors: pb.Authors,
	}

	if pb.Isbn13 != "" {
		m.ISBN13 = &pb.Isbn13
	}
	if pb.CoverUrl != "" {
		m.CoverURL = &pb.CoverUrl
	}
	if pb.Description != "" {
		m.Description = &pb.Description
	}
	if pb.PageCount != 0 {
		pc := int(pb.PageCount)
		m.PageCount = &pc
	}

	return m
}

func int32PtrFromInt16(i *int16) int32 {
	if i == nil {
		return 0
	}
	return int32(*i)
}

func int32FromInt(i int) int32 {
	//nolint:gosec // safe for domain page/percent values
	return int32(i)
}

func int32FromIntPtr(i *int) int32 {
	if i == nil {
		return 0
	}
	return int32FromInt(*i)
}

func protoCompareRef(r *services.CompareRef) *booksv1.BookRef {
	if r == nil {
		return nil
	}
	return &booksv1.BookRef{
		Title:   r.Title,
		Authors: r.Authors,
		Isbn13:  r.ISBN13,
		Status:  r.Status,
		Tags:    r.Tags,
	}
}
