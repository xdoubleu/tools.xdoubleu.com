package services

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/ebookmeta"
)

// DuplicateGroup holds a set of library entries judged to be the same book.
// Entries[0] is the suggested winner (the richest entry to keep).
type DuplicateGroup struct {
	Entries []models.UserBook
	// Reason is the strongest matching signal: "isbn13" | "isbn10" | "title+author"
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

// Status rank constants for winner selection (higher = more progressed).
const (
	statusRankRead    = 3
	statusRankReading = 2
	statusRankToRead  = 1
)

// Richness weight constants — bucket sizes ensure that a higher-weight field
// can never be outweighed by any combination of lower-weight fields.
const (
	richnessStatusWeight   = 1_000_000
	richnessFormatsWeight  = 10_000
	richnessTagsWeight     = 100
	richnessSecondsPerHour = 3600
	// richnessMaxAgeHours caps the age penalty so it never overflows into the
	// tags bucket; 65 535 hours ≈ 7.5 years.
	richnessMaxAgeHours = 65535
)

// Signal strength for duplicate matching (higher = more confident).
const (
	signalISBN13      = 3
	signalISBN10      = 2
	signalTitleAuthor = 1
)

// minDuplicateGroupSize is the minimum group size returned by FindDuplicateGroups.
const minDuplicateGroupSize = 2

// statusRank returns a numeric rank for a UserBook status (higher = more
// progressed). Used to pick the best winner when merging duplicates.
func statusRank(status string) int {
	switch status {
	case models.StatusRead:
		return statusRankRead
	case models.StatusReading:
		return statusRankReading
	case models.StatusToRead:
		return statusRankToRead
	default:
		return 0
	}
}

// richness scores a UserBook for winner selection: higher is better. The
// composite avoids the need for nested sort keys.
func richness(ub models.UserBook) int {
	score := statusRank(ub.Status) * richnessStatusWeight
	score += len(ub.Formats) * richnessFormatsWeight
	score += len(ub.Tags) * richnessTagsWeight
	// Earlier added_at is better (more history); invert by negating unix seconds
	// clamped so it never flips higher-weight buckets.
	seconds := int(ub.AddedAt.Unix())
	if seconds > 0 {
		score -= min(seconds/richnessSecondsPerHour, richnessMaxAgeHours)
	}
	return score
}

// FindDuplicateGroups returns groups of UserBook entries judged to be the same
// book. Two entries are considered duplicates when they share a non-empty
// ISBN13, a non-empty ISBN10, or a normalised title together with at least one
// shared normalised author last name.
//
// Groups of size < 2 are not returned. Within each group Entries[0] is the
// suggested winner (highest richness score; ties broken by BookID to ensure a
// deterministic order).
//
//nolint:cyclop,funlen,gocognit,gocyclo // union-find + winner; cannot split further
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

	signalStrength := map[string]int{
		"isbn13":       signalISBN13,
		"isbn10":       signalISBN10,
		"title+author": signalTitleAuthor,
	}

	union := func(a, b int, sig string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			// already connected — upgrade reason if stronger signal
			if signalStrength[sig] > signalStrength[reason[ra]] {
				reason[ra] = sig
			}
			return
		}
		// merge rb into ra
		parent[rb] = ra
		if signalStrength[sig] > signalStrength[reason[ra]] {
			reason[ra] = sig
		}
	}

	// Pairwise duplicate checks — O(n²) which is fine for typical library sizes.
	for i := 0; i < n; i++ {
		bi := lib[i].Book
		if bi == nil {
			continue
		}
		normTitleI := normalizeTitle(bi.Title)
		authorsI := make(map[string]struct{}, len(bi.Authors))
		for _, a := range bi.Authors {
			if na := normalizeAuthor(a); na != "" {
				authorsI[na] = struct{}{}
			}
		}

		for j := i + 1; j < n; j++ {
			bj := lib[j].Book
			if bj == nil {
				continue
			}

			// ISBN13 match (strongest).
			if bi.ISBN13 != nil && bj.ISBN13 != nil &&
				*bi.ISBN13 != "" && *bi.ISBN13 == *bj.ISBN13 {
				union(i, j, "isbn13")
				continue
			}

			// ISBN10 match.
			if bi.ISBN10 != nil && bj.ISBN10 != nil &&
				*bi.ISBN10 != "" && *bi.ISBN10 == *bj.ISBN10 {
				union(i, j, "isbn10")
				continue
			}

			// Normalised title + shared author last name.
			if normTitleI == "" {
				continue
			}
			normTitleJ := normalizeTitle(bj.Title)
			if normTitleI != normTitleJ {
				continue
			}
			for _, a := range bj.Authors {
				if na := normalizeAuthor(a); na != "" {
					if _, ok := authorsI[na]; ok {
						union(i, j, "title+author")
						break
					}
				}
			}
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
