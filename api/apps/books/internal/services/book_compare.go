package services

import (
	"context"
	"fmt"
	"io"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/books"
)

// DiffMissingInLibrary tags a CSV entry with no match in the library. Shared
// as a constant (rather than a literal) between book_compare.go and
// book_apply.go.
const DiffMissingInLibrary = "missing-in-library"

// DiffTags tags a matched pair whose tag sets differ. Shared as a constant
// between book_compare.go and book_apply.go.
const DiffTags = "tags"

// CompareRef is a lightweight snapshot of one book used in comparison results.
type CompareRef struct {
	Title   string
	Authors []string
	ISBN13  string
	Status  string
	Tags    []string
}

// CompareMismatch describes one pair of entries that differ between the CSV
// and the library. CSV or Library may be nil when the book only exists on one side.
type CompareMismatch struct {
	CSV     *CompareRef
	Library *CompareRef
	// Differences lists active tags: "missing-in-library" | "missing-in-csv" |
	// "status" | "isbn" | "title"
	Differences []string
	// ID is a stable per-row key for ApplyCSVFix: the library book_id when the
	// book is matched or library-only, or "csv:<index>" when it only exists in
	// the CSV.
	ID string
	// LibBook is the matched or library-only entry, used by ApplyCSVFix to
	// know which library row to update. Nil for CSV-only rows.
	LibBook *models.UserBook
	// CSVEntry is the parsed CSV row, used by ApplyCSVFix as the source of
	// truth for the fix. Nil for library-only rows.
	CSVEntry *books.ParsedEntry
}

// CompareResult is the output of a CSV-vs-library comparison.
type CompareResult struct {
	CSVCount     int
	LibraryCount int
	MatchedCount int
	Mismatches   []CompareMismatch
}

// CompareCSV parses a Goodreads CSV export and diffs it against the authenticated
// user's library. It is read-only — no data is written.
func (s *BookService) CompareCSV(
	ctx context.Context,
	userID string,
	r io.Reader,
) (CompareResult, error) {
	entries, err := books.ParseCSV(r)
	if err != nil {
		return CompareResult{}, err
	}

	lib, err := s.books.GetLibrary(ctx, userID)
	if err != nil {
		return CompareResult{}, err
	}

	return CompareWithCSV(entries, lib), nil
}

// CompareWithCSV is the pure diff logic; exported so it can be unit-tested
// directly without a database.
//
// Matching priority mirrors the import upsert and FindDuplicateGroups:
//  1. Normalized ISBN-13 (if both sides have one).
//  2. Normalized title + any overlapping author last-name.
//
// A matched pair is only included in Mismatches when at least one field differs.
// Unmatched CSV entries get "missing-in-library"; unmatched library entries get
// "missing-in-csv".
//
// stays together; gosec G602 false-positives on &entries[entryIdx]/&lib[i] —
// entryIdx/i/libIdx are always range- or bounds-checked indices into the same
// slice.
//
//nolint:cyclop,gocognit,gocyclo,funlen,gosec // matching loop; branches; logic
func CompareWithCSV(
	entries []books.ParsedEntry,
	lib []models.UserBook,
) CompareResult {
	// Pre-normalize the library once — O(n).
	type libNorm struct {
		isbn   string      // normalizeISBN of book.ISBN13
		title  string      // normalizeTitle of book.Title
		tokens []string    // titleTokens of book.Title, for fuzzy fallback
		lastns []string    // normalizeAuthor last-names
		ref    *CompareRef // pointer into libRefs (set after slice build)
		idx    int         // index in lib
	}
	libRefs := make([]CompareRef, len(lib))
	norms := make([]libNorm, len(lib))
	for i, ub := range lib {
		isbn := ""
		title := ""
		var tokens []string
		var lastns []string
		if ub.Book != nil {
			if ub.Book.ISBN13 != nil {
				isbn = normalizeISBN(*ub.Book.ISBN13)
			}
			title = normalizeTitle(ub.Book.Title)
			tokens = titleTokens(ub.Book.Title)
			lastns = make([]string, 0, len(ub.Book.Authors))
			for _, a := range ub.Book.Authors {
				if n := normalizeAuthor(a); n != "" {
					lastns = append(lastns, n)
				}
			}
		}
		libRefs[i] = CompareRef{
			Title: func() string {
				if ub.Book != nil {
					return ub.Book.Title
				}
				return ""
			}(),
			Authors: func() []string {
				if ub.Book != nil {
					return ub.Book.Authors
				}
				return nil
			}(),
			ISBN13: func() string {
				if ub.Book != nil && ub.Book.ISBN13 != nil {
					return *ub.Book.ISBN13
				}
				return ""
			}(),
			Status: ub.Status,
			Tags:   ub.Tags,
		}
		norms[i] = libNorm{
			isbn:   isbn,
			title:  title,
			tokens: tokens,
			lastns: lastns,
			ref:    &libRefs[i],
			idx:    i,
		}
	}

	// Build indexes.
	isbnIdx := make(map[string]int, len(norms)) // normalISBN → lib index
	for i, n := range norms {
		if n.isbn != "" {
			isbnIdx[n.isbn] = i
		}
	}

	// Pre-build title→[]libIndex for O(1) candidate lookup.
	titleIdx := make(map[string][]int, len(norms))
	for i, n := range norms {
		if n.title != "" {
			titleIdx[n.title] = append(titleIdx[n.title], i)
		}
	}

	matched := make([]bool, len(lib)) // which lib entries got matched
	var mismatches []CompareMismatch
	matchedCount := 0

	for entryIdx := range entries {
		entry := entries[entryIdx]
		csvRef := CompareRef{
			Title:   entry.Book.Title,
			Authors: entry.Book.Authors,
			ISBN13: func() string {
				if entry.Book.ISBN13 != nil {
					return *entry.Book.ISBN13
				}
				return ""
			}(),
			Status: entry.UserBook.Status,
			Tags:   entry.UserBook.Tags,
		}

		csvISBN := ""
		if entry.Book.ISBN13 != nil {
			csvISBN = normalizeISBN(*entry.Book.ISBN13)
		}
		csvTitle := normalizeTitle(entry.Book.Title)
		csvTokens := titleTokens(entry.Book.Title)
		csvLastns := make(map[string]struct{}, len(entry.Book.Authors))
		for _, a := range entry.Book.Authors {
			if n := normalizeAuthor(a); n != "" {
				csvLastns[n] = struct{}{}
			}
		}

		// Match by ISBN first, then by exact title+author, then by fuzzy
		// title (same author, similar-enough title tokens) — mirrors
		// FindDuplicateGroups.
		libIdx := -1
		if csvISBN != "" {
			if i, ok := isbnIdx[csvISBN]; ok {
				libIdx = i
			}
		}
		if libIdx == -1 && csvTitle != "" && len(csvLastns) > 0 {
			for _, i := range titleIdx[csvTitle] {
				for _, ln := range norms[i].lastns {
					if _, ok := csvLastns[ln]; ok {
						libIdx = i
						break
					}
				}
				if libIdx != -1 {
					break
				}
			}
		}
		if libIdx == -1 && len(csvTokens) > 0 && len(csvLastns) > 0 {
			for i, n := range norms {
				if matched[i] {
					continue
				}
				hasSharedAuthor := false
				for _, ln := range n.lastns {
					if _, ok := csvLastns[ln]; ok {
						hasSharedAuthor = true
						break
					}
				}
				if hasSharedAuthor && titlesFuzzyMatch(csvTokens, n.tokens) {
					libIdx = i
					break
				}
			}
		}

		if libIdx == -1 {
			// No match in library.
			ref := csvRef
			mismatches = append(
				mismatches,
				CompareMismatch{ //nolint:exhaustruct //Library nil by design
					CSV:         &ref,
					Differences: []string{DiffMissingInLibrary},
					ID:          fmt.Sprintf("csv:%d", entryIdx),
					CSVEntry:    &entries[entryIdx],
				},
			)
			continue
		}

		matched[libIdx] = true
		matchedCount++

		// Compute diffs on the matched pair.
		libRef := norms[libIdx].ref
		var diffs []string

		if csvRef.Status != libRef.Status {
			diffs = append(diffs, "status")
		}

		csvISBNNorm := normalizeISBN(csvRef.ISBN13)
		libISBNNorm := normalizeISBN(libRef.ISBN13)
		if csvISBNNorm != libISBNNorm {
			diffs = append(diffs, "isbn")
		}

		if normalizeTitle(csvRef.Title) != normalizeTitle(libRef.Title) {
			diffs = append(diffs, "title")
		}

		if !sameTagSet(csvRef.Tags, libRef.Tags) {
			diffs = append(diffs, DiffTags)
		}

		if len(diffs) > 0 {
			cr := csvRef
			lr := *libRef
			mismatches = append(mismatches, CompareMismatch{
				CSV:         &cr,
				Library:     &lr,
				Differences: diffs,
				ID:          lib[libIdx].BookID.String(),
				LibBook:     &lib[libIdx],
				CSVEntry:    &entries[entryIdx],
			})
		}
	}

	// Library entries that were never matched.
	for i, was := range matched {
		if !was {
			lr := libRefs[i]
			mismatches = append(
				mismatches,
				CompareMismatch{ //nolint:exhaustruct //CSV nil by design
					Library:     &lr,
					Differences: []string{"missing-in-csv"},
					ID:          lib[i].BookID.String(),
					LibBook:     &lib[i],
				},
			)
		}
	}

	return CompareResult{
		CSVCount:     len(entries),
		LibraryCount: len(lib),
		MatchedCount: matchedCount,
		Mismatches:   mismatches,
	}
}

// sameTagSet reports whether two tag lists contain the same set of tags,
// ignoring order and duplicates.
func sameTagSet(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[t] = struct{}{}
	}
	for _, t := range b {
		if _, ok := set[t]; !ok {
			return false
		}
		delete(set, t)
	}
	return len(set) == 0
}
