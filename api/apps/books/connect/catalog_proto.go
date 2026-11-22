package connect

import (
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func protoSourceProposal(p services.SourceProposal) *booksv1.SourceBook {
	return &booksv1.SourceBook{
		Source:      p.Source,
		CoverUrl:    p.CoverURL,
		Description: p.Description,
		PageCount:   Int32FromInt(p.PageCount),
		Isbn13:      p.ISBN13,
		Title:       p.Title,
		Authors:     p.Authors,
		Differs:     p.Differs,
		Index:       Int32FromInt(p.Index),

		SeriesName:     p.SeriesName,
		SeriesPosition: p.SeriesPosition,
	}
}

// protoResyncProposal maps a services.ResyncProposal to the admin-facing
// proto view used by the resync wizard.
func protoResyncProposal(p services.ResyncProposal) *booksv1.ResyncProposal {
	sources := make([]*booksv1.SourceBook, len(p.Sources))
	for i, sp := range p.Sources {
		sources[i] = protoSourceProposal(sp)
	}
	return &booksv1.ResyncProposal{
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
	m.Series = seriesFromProto(pb.SeriesName, pb.SeriesPosition)

	return m
}

func int32PtrFromInt16(i *int16) int32 {
	if i == nil {
		return 0
	}
	return int32(*i)
}

// Int32FromInt narrows a domain count to the wire int32.
func Int32FromInt(i int) int32 {
	//nolint:gosec // safe for domain page/percent values
	return int32(i)
}

func int32FromIntPtr(i *int) int32 {
	if i == nil {
		return 0
	}
	return Int32FromInt(*i)
}

// Source names shared by the source-stats and exact-sources RPCs; they must
// match repositories.sourceColumns' keys.
const (
	sourceUniCat    = "unicat"
	sourceHardcover = "hardcover"
)

// SourceStatsResponse maps the catalog's per-source coverage onto the wire
// type.
func SourceStatsResponse(stats *services.SourceStats) *booksv1.GetSourceStatsResponse {
	return &booksv1.GetSourceStatsResponse{
		Sources: []*booksv1.SourceStat{
			{
				Source:      sourceUniCat,
				FoundCount:  Int32FromInt(stats.UniCatFound),
				UniqueCount: Int32FromInt(stats.UniCatUnique),
				MissedCount: Int32FromInt(stats.UniCatMissed),
			},
			{
				Source:      sourceHardcover,
				FoundCount:  Int32FromInt(stats.HardcoverFound),
				UniqueCount: Int32FromInt(stats.HardcoverUnique),
				MissedCount: Int32FromInt(stats.HardcoverMissed),
			},
		},
		TotalBooks:       Int32FromInt(stats.TotalBooks),
		NotFoundAnywhere: Int32FromInt(stats.NotFoundAnywhere),
		NeverScanned:     Int32FromInt(stats.NeverScanned),
		Overlaps:         sourceOverlaps(stats),
		MissedOverlaps:   sourceMissedOverlaps(stats),
	}
}

// sourceOverlaps lists the "found by both sources" combo. Zero-count is kept
// — the web report filters it out.
func sourceOverlaps(stats *services.SourceStats) []*booksv1.SourceComboStat {
	uc, hc := sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		{Sources: []string{uc, hc}, Count: Int32FromInt(stats.Both)},
	}
}

// sourceMissedOverlaps mirrors sourceOverlaps: "missed by both sources" (both
// confirmed IS FALSE).
func sourceMissedOverlaps(
	stats *services.SourceStats,
) []*booksv1.SourceComboStat {
	uc, hc := sourceUniCat, sourceHardcover
	return []*booksv1.SourceComboStat{
		{Sources: []string{uc, hc}, Count: Int32FromInt(stats.BothMissed)},
	}
}

// DuplicatesResponse maps duplicate groups onto the wire type.
func DuplicatesResponse(
	groups []services.DuplicateGroup,
	coverBaseURL string,
) *booksv1.FindDuplicatesResponse {
	protoGroups := make([]*booksv1.DuplicateGroup, len(groups))
	for i, g := range groups {
		entries := make([]*booksv1.UserBook, len(g.Entries))
		for j, e := range g.Entries {
			entries[j] = ProtoUserBook(e, coverBaseURL)
		}
		protoGroups[i] = &booksv1.DuplicateGroup{
			Entries: entries,
			Reason:  g.Reason,
		}
	}
	return &booksv1.FindDuplicatesResponse{Groups: protoGroups}
}

// BooksInExactSourcesResponse maps catalog books onto the wire type.
func BooksInExactSourcesResponse(
	books []models.Book,
	coverBaseURL string,
) *booksv1.ListBooksInExactSourcesResponse {
	out := make([]*booksv1.Book, len(books))
	for i := range books {
		out[i] = protoBook(&books[i], coverBaseURL)
	}
	return &booksv1.ListBooksInExactSourcesResponse{Books: out}
}

// ResyncProposalsResponse maps resync proposals onto the wire type.
func ResyncProposalsResponse(
	proposals []services.ResyncProposal,
) *booksv1.ListResyncProposalsResponse {
	result := make([]*booksv1.ResyncProposal, len(proposals))
	for i, p := range proposals {
		result[i] = protoResyncProposal(p)
	}
	return &booksv1.ListResyncProposalsResponse{Proposals: result}
}

// BookSourcesResponse maps one book's source candidates onto the wire type.
func BookSourcesResponse(p services.ResyncProposal) *booksv1.GetBookSourcesResponse {
	return &booksv1.GetBookSourcesResponse{Proposal: protoResyncProposal(p)}
}
