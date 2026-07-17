package reading

import (
	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

const isbn13Length = 13

// protoSourceProposal maps a services.SourceProposal to its proto view.
func protoSourceProposal(p services.SourceProposal) *readingv1.SourceBook {
	return &readingv1.SourceBook{
		Source:      p.Source,
		CoverUrl:    p.CoverURL,
		Description: p.Description,
		PageCount:   int32FromInt(p.PageCount),
		Isbn13:      p.ISBN13,
		Title:       p.Title,
		Authors:     p.Authors,
		Differs:     p.Differs,
		Index:       int32FromInt(p.Index),
	}
}

// protoResyncProposal maps a services.ResyncProposal to the admin-facing
// proto view used by the resync wizard.
func protoResyncProposal(p services.ResyncProposal) *readingv1.ResyncProposal {
	sources := make([]*readingv1.SourceBook, len(p.Sources))
	for i, sp := range p.Sources {
		sources[i] = protoSourceProposal(sp)
	}
	return &readingv1.ResyncProposal{
		BookId:  p.BookID,
		Library: protoSourceProposal(p.Library),
		Sources: sources,
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
func protoBookToModel(pb *readingv1.Book) *models.Book {
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
