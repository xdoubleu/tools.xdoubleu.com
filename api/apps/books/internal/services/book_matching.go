package services

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
)

// DuplicateGroup holds a set of library entries judged to be the same book.
// Entries[0] is the suggested winner — the entry with the most complete book
// metadata (cover, description, page count, ISBNs), since that is the one
// attribute MergeBooks does not consolidate.
type DuplicateGroup struct {
	Entries []models.UserBook
	// Reason is the strongest matching signal: "isbn13" | "title+author"
	Reason string
}

// normalizeTitle lower-cases s, folds diacritics, drops everything after the
// first colon (subtitle), and strips all non-alphanumeric runes. Returns ""
// when the result is empty so callers can skip matching on garbage metadata.
func normalizeTitle(s string) string {
	// Strip subtitle — take only the part before the first colon.
	if idx := strings.IndexByte(s, ':'); idx >= 0 {
		s = s[:idx]
	}
	return normalizeString(s)
}

// normalizeAuthor lower-cases s, folds diacritics, and reduces the name to its
// last-name token. It handles two common formats:
//   - "Last, First…" (comma present) → everything before the first comma
//   - "First… Last"  (no comma)      → the last whitespace-delimited token
//
// Returns "" on empty input.
func normalizeAuthor(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		// "Tolkien, J.R.R." → "Tolkien"
		s = s[:idx]
	} else {
		// "J.R.R. Tolkien" → "Tolkien"
		parts := strings.Fields(s)
		if len(parts) > 0 {
			s = parts[len(parts)-1]
		}
	}
	return normalizeString(s)
}

// normalizeISBN strips all non-digit characters from s so that formatted
// ("978-94-6310-738-9") and plain ("9789463107389") representations of the
// same ISBN compare equal. Returns "" when s contains no digits.
func normalizeISBN(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeString lower-cases s, folds diacritics (NFD → strip non-spacing
// marks → NFC), and removes all non-alphanumeric runes.
func normalizeString(s string) string {
	// NFD decomposition, strip combining marks, NFC recomposition.
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	folded, _, _ := transform.String(t, s)

	var b strings.Builder
	for _, r := range strings.ToLower(folded) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Status rank constants for merge-time consolidation (higher wins).
// A custom shelf outranks every built-in status because the user deliberately
// organised the book there.  dropped is ranked lowest so it never overrides any
// other placement.
const (
	statusRankShelf   = 4
	statusRankRead    = 3
	statusRankReading = 2
	statusRankToRead  = 1
	statusRankDropped = 0
)

// Richness weight constants — bucket sizes ensure that a higher-weight field
// can never be outweighed by any combination of lower-weight fields.
//
// Priority order: metadata completeness > reading status > tags > age.
// Formats are intentionally excluded: MergeBooks repoints all file formats
// onto the winner regardless, so format count should not drive winner selection.
const (
	richnessCompletenessWeight = 100_000_000
	richnessStatusWeight       = 1_000_000
	richnessTagsWeight         = 100
	richnessSecondsPerHour     = 3600
	// richnessMaxAgeHours caps the age penalty so it never overflows into the
	// tags bucket; 65 535 hours ≈ 7.5 years.
	richnessMaxAgeHours = 65535
)

// Signal strength for duplicate matching (higher = more confident).
const (
	signalISBN13      = 2
	signalTitleAuthor = 1
)

// minDuplicateGroupSize is the minimum group size returned by FindDuplicateGroups.
const minDuplicateGroupSize = 2

// statusRank returns a numeric rank for a UserBook status used during merge
// consolidation (higher wins).  Custom shelves (any non-built-in, non-empty
// status) outrank every built-in status so intentional shelf placement is
// preserved.  An empty string returns 0 so a missing status never beats a real
// one.
func statusRank(status string) int {
	switch status {
	case models.StatusRead:
		return statusRankRead
	case models.StatusReading:
		return statusRankReading
	case models.StatusToRead:
		return statusRankToRead
	case models.StatusDropped:
		return statusRankDropped
	case "":
		return 0
	default:
		// Any non-empty, non-built-in status is a custom shelf.
		return statusRankShelf
	}
}

// metadataCompleteness counts how many catalog Book fields are populated.
// It is the dominant factor in richness so that the entry carrying the most
// complete metadata is suggested as the merge winner — metadata is the one
// attribute MergeBooks does not consolidate across duplicates.
// Returns 0 when book is nil.
func metadataCompleteness(book *models.Book) int {
	if book == nil {
		return 0
	}
	score := 0
	if len(book.Authors) > 0 {
		score++
	}
	if book.ISBN13 != nil && *book.ISBN13 != "" {
		score++
	}
	if book.CoverURL != nil && *book.CoverURL != "" {
		score++
	}
	if book.Description != nil && *book.Description != "" {
		score++
	}
	if book.PageCount != nil && *book.PageCount > 0 {
		score++
	}
	return score
}

// richness scores a UserBook for winner selection: higher is better. The
// composite avoids the need for nested sort keys.
//
// Priority: metadata completeness > reading status > tags > age.
// Formats are excluded — MergeBooks repoints all file formats onto the winner.
func richness(ub models.UserBook) int {
	score := metadataCompleteness(ub.Book) * richnessCompletenessWeight
	score += statusRank(ub.Status) * richnessStatusWeight
	score += len(ub.Tags) * richnessTagsWeight
	// Earlier added_at is better (more history); invert by negating unix seconds
	// clamped so it never flips higher-weight buckets.
	seconds := int(ub.AddedAt.Unix())
	if seconds > 0 {
		score -= min(seconds/richnessSecondsPerHour, richnessMaxAgeHours)
	}
	return score
}

// signalStrengthFor returns the numeric strength of a duplicate-matching
// reason string. Higher is more confident. Unknown reasons return 0.
func signalStrengthFor(reason string) int {
	switch reason {
	case "isbn13":
		return signalISBN13
	case "title+author":
		return signalTitleAuthor
	default:
		return 0
	}
}

// FindDuplicateGroups returns groups of UserBook entries judged to be the same
// book. Two entries are considered duplicates when they share a non-empty
// ISBN13, or a normalised title together with at least one shared normalised
// author last name.
//
// Groups of size < 2 are not returned. Within each group Entries[0] is the
// suggested winner (most complete metadata; ties broken by status, then tags,
// then age, then BookID to ensure a deterministic order). The returned group
// list itself is sorted by matching-signal strength descending (isbn13 first,
// then title+author), then by the winner's title ascending, then by the
// winner's BookID as a final unique tiebreak — so the order is stable across
// repeated calls regardless of the input slice ordering.
//
//nolint:funlen,gocognit,gocyclo,cyclop // union-find + buckets + winner; cannot split
func FindDuplicateGroups(lib []models.UserBook) []DuplicateGroup {
	n := len(lib)
	if n < minDuplicateGroupSize {
		return nil
	}

	// Union-find parent array (index into lib).
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	// reason records the strongest signal that caused a union.
	reason := make([]string, n)

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	union := func(a, b int, sig string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			// already connected — upgrade reason if stronger signal
			if signalStrengthFor(sig) > signalStrengthFor(reason[ra]) {
				reason[ra] = sig
			}
			return
		}
		// merge rb into ra
		parent[rb] = ra
		if signalStrengthFor(sig) > signalStrengthFor(reason[ra]) {
			reason[ra] = sig
		}
	}

	// Precompute normalised fields once per book — O(n).
	type bookNorm struct {
		isbn13  string
		title   string
		authors []string // normalised last names
	}
	norms := make([]bookNorm, n)
	for i, ub := range lib {
		b := ub.Book
		if b == nil {
			continue
		}
		var bn bookNorm
		if b.ISBN13 != nil {
			bn.isbn13 = normalizeISBN(*b.ISBN13)
		}
		bn.title = normalizeTitle(b.Title)
		bn.authors = make([]string, 0, len(b.Authors))
		for _, a := range b.Authors {
			if na := normalizeAuthor(a); na != "" {
				bn.authors = append(bn.authors, na)
			}
		}
		norms[i] = bn
	}

	// Build key→index buckets and union within each bucket — O(n) overall.
	// ISBN13 bucket: one entry per non-empty ISBN-13 value.
	// Title+author bucket: one entry per (normTitle, normLastName) pair so that
	// two books match when they share a normalised title AND at least one author
	// last name — identical semantics to the original pairwise check.
	isbn13Bucket := make(map[string][]int, n)
	titleAuthorBucket := make(map[string][]int, n)

	for i, bn := range norms {
		if lib[i].Book == nil {
			continue
		}
		if bn.isbn13 != "" {
			isbn13Bucket[bn.isbn13] = append(isbn13Bucket[bn.isbn13], i)
		}
		if bn.title != "" {
			for _, a := range bn.authors {
				key := bn.title + "\x00" + a
				titleAuthorBucket[key] = append(titleAuthorBucket[key], i)
			}
		}
	}

	for _, members := range isbn13Bucket {
		for k := 1; k < len(members); k++ {
			union(members[0], members[k], "isbn13")
		}
	}
	for _, members := range titleAuthorBucket {
		for k := 1; k < len(members); k++ {
			union(members[0], members[k], "title+author")
		}
	}

	// Collect groups by root.
	groups := make(map[int][]int)
	for i := range lib {
		r := find(i)
		groups[r] = append(groups[r], i)
	}

	result := make([]DuplicateGroup, 0, len(groups))
	for root, members := range groups {
		if len(members) < minDuplicateGroupSize {
			continue
		}
		// Sort members: highest richness first; break ties by BookID string for
		// determinism.
		slices.SortFunc(members, func(a, b int) int {
			ra, rb := richness(lib[a]), richness(lib[b])
			if ra != rb {
				return rb - ra // descending richness
			}
			sa := lib[a].BookID.String()
			sb := lib[b].BookID.String()
			if sa < sb {
				return -1
			}
			if sa > sb {
				return 1
			}
			return 0
		})
		entries := make([]models.UserBook, len(members))
		for k, idx := range members {
			entries[k] = lib[idx]
		}
		result = append(result, DuplicateGroup{
			Entries: entries,
			Reason:  reason[root],
		})
	}

	if len(result) == 0 {
		return nil
	}

	// Sort the group list so repeated calls return a stable order:
	// 1. Matching-signal strength descending (isbn13 > isbn10 > title+author).
	// 2. Winner book title ascending (case-insensitive).
	// 3. Winner BookID string as final tiebreak (unique).
	slices.SortFunc(result, func(a, b DuplicateGroup) int {
		sa := signalStrengthFor(a.Reason)
		sb := signalStrengthFor(b.Reason)
		if sa != sb {
			return sb - sa // descending
		}
		titleA := ""
		if len(a.Entries) > 0 && a.Entries[0].Book != nil {
			titleA = strings.ToLower(a.Entries[0].Book.Title)
		}
		titleB := ""
		if len(b.Entries) > 0 && b.Entries[0].Book != nil {
			titleB = strings.ToLower(b.Entries[0].Book.Title)
		}
		if titleA != titleB {
			if titleA < titleB {
				return -1
			}
			return 1
		}
		idA := ""
		if len(a.Entries) > 0 {
			idA = a.Entries[0].BookID.String()
		}
		idB := ""
		if len(b.Entries) > 0 {
			idB = b.Entries[0].BookID.String()
		}
		if idA < idB {
			return -1
		}
		if idA > idB {
			return 1
		}
		return 0
	})

	return result
}

// matchLibraryByMetadata scans lib for a book whose normalized title matches
// the file metadata and whose author set shares at least one normalized last
// name with the file authors. Returns nil when:
//   - the file title normalizes to "" (empty/garbage metadata)
//   - the file has no authors (cannot verify authorship)
//   - no library entry satisfies both conditions
func matchLibraryByMetadata(
	lib []models.UserBook,
	meta ebookmeta.Metadata,
) *models.UserBook {
	fileTitle := normalizeTitle(meta.Title)
	if fileTitle == "" || len(meta.Authors) == 0 {
		return nil
	}

	// Build the set of normalized last names from the file's author list.
	fileAuthors := make(map[string]struct{}, len(meta.Authors))
	for _, a := range meta.Authors {
		if n := normalizeAuthor(a); n != "" {
			fileAuthors[n] = struct{}{}
		}
	}
	if len(fileAuthors) == 0 {
		return nil
	}

	for i := range lib {
		ub := &lib[i]
		if ub.Book == nil {
			continue
		}
		if normalizeTitle(ub.Book.Title) != fileTitle {
			continue
		}
		// Title matched — check for any author overlap.
		for _, a := range ub.Book.Authors {
			if _, ok := fileAuthors[normalizeAuthor(a)]; ok {
				return ub
			}
		}
	}
	return nil
}
