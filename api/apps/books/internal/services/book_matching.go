package services

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
)

// DuplicateGroup holds library entries judged to be the same book. Entries[0]
// is the suggested winner: the most complete metadata, which MergeBooks does
// not consolidate.
type DuplicateGroup struct {
	Entries []models.UserBook
	// Reason is the strongest matching signal: "isbn13" | "title+author"
	Reason string
}

// parentheticalRe matches series/edition annotations like "(Series, #1)".
var parentheticalRe = regexp.MustCompile(`[(\[][^)\]]*[)\]]`)

// volumeNumberRe matches a keyword-marked volume number ("Vol. 2", "Part 1").
// A bare "#1" has no keyword and stays stripped as noise.
var volumeNumberRe = regexp.MustCompile(
	`(?i)\b(?:volume|vol|book|part|edition|ed)\.?\s*#?\s*(\d+)`,
)

func volumeNumbers(s string) []string {
	matches := volumeNumberRe.FindAllStringSubmatch(s, -1)
	nums := make([]string, 0, len(matches))
	for _, m := range matches {
		nums = append(nums, m[1])
	}
	return nums
}

// stripAnnotations drops subtitle/series/edition noise (after ':', ';', " - ",
// and bracketed segments), re-appending any stripped volume number so distinct
// volumes never normalize the same.
func stripAnnotations(s string) string {
	raw := s
	main := s
	if idx := strings.IndexByte(main, ':'); idx >= 0 {
		main = main[:idx]
	}
	if idx := strings.IndexByte(main, ';'); idx >= 0 {
		main = main[:idx]
	}
	if idx := strings.Index(main, " - "); idx >= 0 {
		main = main[:idx]
	}
	main = strings.TrimSpace(parentheticalRe.ReplaceAllString(main, ""))

	if lost := lostVolumeNumbers(raw, main); lost != "" {
		main = strings.TrimSpace(main + " " + lost)
	}
	return main
}

// lostVolumeNumbers returns the volume numbers in raw but not main (multiset
// difference), space-joined.
func lostVolumeNumbers(raw, main string) string {
	mainCounts := map[string]int{}
	for _, n := range volumeNumbers(main) {
		mainCounts[n]++
	}
	var missing []string
	for _, n := range volumeNumbers(raw) {
		if mainCounts[n] > 0 {
			mainCounts[n]--
			continue
		}
		missing = append(missing, n)
	}
	return strings.Join(missing, " ")
}

func isLeadingArticle(w string) bool {
	return strings.EqualFold(w, "the") || strings.EqualFold(w, "a") ||
		strings.EqualFold(w, "an")
}

func stripLeadingArticle(s string) string {
	fields := strings.Fields(s)
	if len(fields) > 1 && isLeadingArticle(fields[0]) {
		return strings.Join(fields[1:], " ")
	}
	return s
}

// normalizeTitle folds case and diacritics, strips annotations, a leading
// article and non-alphanumerics. Returns "" for garbage metadata.
func normalizeTitle(s string) string {
	s = stripAnnotations(s)
	s = stripLeadingArticle(s)
	return normalizeString(s)
}

// titleTokens is normalizeTitle but keeps word boundaries, for fuzzy matching.
func titleTokens(s string) []string {
	s = stripAnnotations(s)
	words := strings.Fields(s)
	tokens := make([]string, 0, len(words))
	for i, w := range words {
		if i == 0 && isLeadingArticle(w) {
			continue
		}
		if nw := normalizeString(w); nw != "" {
			tokens = append(tokens, nw)
		}
	}
	return tokens
}

// tokenSimilarity returns the Jaccard similarity of two token sets (0 if
// either is empty).
func tokenSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(a))
	for _, t := range a {
		setA[t] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, t := range b {
		setB[t] = struct{}{}
	}
	intersection := 0
	for t := range setA {
		if _, ok := setB[t]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func isNumericToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// romanNumeralRe matches only canonical numerals so words like "civil" are
// rejected; the empty match is guarded in isEnumeratorToken.
var romanNumeralRe = regexp.MustCompile(
	`(?i)^m{0,3}(?:c[md]|d?c{0,3})(?:x[cl]|l?x{0,3})(?:i[xv]|v?i{0,3})$`,
)

func isEnumeratorToken(s string) bool {
	return s != "" && (isNumericToken(s) || romanNumeralRe.MatchString(s))
}

func enumeratorTokens(tokens []string) []string {
	var nums []string
	for _, t := range tokens {
		if isEnumeratorToken(t) {
			nums = append(nums, t)
		}
	}
	return nums
}

// enumeratorTokensDiffer reports whether a and b disagree on their ordered
// volume/edition tokens: "Book 1" vs "Book 2" must never fuzzy-match, and
// "Book 1 Edition 2" vs "Book 2 Edition 1" differ too.
func enumeratorTokensDiffer(a, b []string) bool {
	na, nb := enumeratorTokens(a), enumeratorTokens(b)
	if len(na) != len(nb) {
		return true
	}
	for i := range na {
		if na[i] != nb[i] {
			return true
		}
	}
	return false
}

func titlesFuzzyMatch(a, b []string) bool {
	return tokenSimilarity(a, b) >= titleSimilarityThreshold &&
		!enumeratorTokensDiffer(a, b)
}

// titleSimilarityThreshold: "The Fellowship of the Ring" matches its reordered
// form (1.0) but not "The Return of the King" (~0.33).
const titleSimilarityThreshold = 0.7

// normalizeAuthor reduces a name to its folded last name, from either
// "Last, First" or "First Last".
func normalizeAuthor(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		s = s[:idx]
	} else {
		parts := strings.Fields(s)
		if len(parts) > 0 {
			s = parts[len(parts)-1]
		}
	}
	return normalizeString(s)
}

// normalizeISBN strips non-digits so formatted and plain ISBNs compare equal.
func normalizeISBN(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeString folds case and diacritics and drops non-alphanumerics.
func normalizeString(s string) string {
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

// Status ranks for merge consolidation (higher wins). A custom shelf is a
// deliberate placement so outranks built-ins; dropped ranks lowest.
const (
	statusRankShelf   = 4
	statusRankRead    = 3
	statusRankReading = 2
	statusRankToRead  = 1
	statusRankDropped = 0
)

// Richness weights: bucket sizes ensure a higher-weight field is never
// outweighed by lower ones. Metadata > status > tags > age; formats are
// excluded since MergeBooks repoints every format anyway.
const (
	richnessCompletenessWeight = 100_000_000
	richnessStatusWeight       = 1_000_000
	richnessTagsWeight         = 100
	richnessSecondsPerHour     = 3600
	// richnessMaxAgeHours keeps the age penalty out of the tags bucket (~7.5 years).
	richnessMaxAgeHours = 65535
)

// Signal strength for duplicate matching (higher = more confident).
const (
	signalISBN13      = 2
	signalTitleAuthor = 1
)

const minDuplicateGroupSize = 2

// statusRank ranks a status for merge consolidation (higher wins); "" is 0.
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
		return statusRankShelf
	}
}

// metadataCompleteness counts populated Book fields; 0 for nil.
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

// richness scores a UserBook for winner selection (higher is better).
func richness(ub models.UserBook) int {
	score := metadataCompleteness(ub.Book) * richnessCompletenessWeight
	score += statusRank(ub.Status) * richnessStatusWeight
	score += len(ub.Tags) * richnessTagsWeight
	// Earlier added_at wins; clamped so it never flips higher-weight buckets.
	seconds := int(ub.AddedAt.Unix())
	if seconds > 0 {
		score -= min(seconds/richnessSecondsPerHour, richnessMaxAgeHours)
	}
	return score
}

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

// FindDuplicateGroups groups entries sharing an ISBN13, or a normalised title
// (exact or fuzzy, see titlesFuzzyMatch) plus an author last name. Entries[0]
// is the richest; groups are sorted by signal strength, winner title, then
// BookID so the order is deterministic.
//
//nolint:funlen,gocognit,gocyclo,cyclop // union-find + buckets + winner; cannot split
func FindDuplicateGroups(lib []models.UserBook) []DuplicateGroup {
	n := len(lib)
	if n < minDuplicateGroupSize {
		return nil
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
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
			if signalStrengthFor(sig) > signalStrengthFor(reason[ra]) {
				reason[ra] = sig
			}
			return
		}
		parent[rb] = ra
		if signalStrengthFor(sig) > signalStrengthFor(reason[ra]) {
			reason[ra] = sig
		}
	}

	type bookNorm struct {
		isbn13  string
		title   string
		tokens  []string // fuzzy-matching title tokens
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
		bn.tokens = titleTokens(b.Title)
		bn.authors = make([]string, 0, len(b.Authors))
		for _, a := range b.Authors {
			if na := normalizeAuthor(a); na != "" {
				bn.authors = append(bn.authors, na)
			}
		}
		norms[i] = bn
	}

	// Bucket by ISBN13 and by (title, author last name), unioning within buckets.
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

	// Fuzzy pass: pairwise within each author bucket. Fine at personal-library
	// scale; add a blocking index if that ever changes.
	authorBucket := make(map[string][]int, n)
	for i, bn := range norms {
		if lib[i].Book == nil {
			continue
		}
		for _, a := range bn.authors {
			authorBucket[a] = append(authorBucket[a], i)
		}
	}
	for _, members := range authorBucket {
		for x := 1; x < len(members); x++ {
			for y := range x {
				a, b := members[x], members[y]
				if find(a) == find(b) {
					continue // already grouped
				}
				if titlesFuzzyMatch(norms[a].tokens, norms[b].tokens) {
					union(a, b, "title+author")
				}
			}
		}
	}

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

// matchLibraryByMetadata returns the entry whose normalized title matches and
// which shares an author last name, or nil (also when the file has no title
// or no authors).
func matchLibraryByMetadata(
	lib []models.UserBook,
	meta ebookmeta.Metadata,
) *models.UserBook {
	fileTitle := normalizeTitle(meta.Title)
	if fileTitle == "" || len(meta.Authors) == 0 {
		return nil
	}

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
		for _, a := range ub.Book.Authors {
			if _, ok := fileAuthors[normalizeAuthor(a)]; ok {
				return ub
			}
		}
	}
	return nil
}

// matchCatalogByMetadata adds a fuzzy fallback (FindDuplicateGroups' title
// similarity) to matchLibraryByMetadata so a reordered or subtitle-variant
// title still resolves to an existing catalog entry.
func matchCatalogByMetadata(
	catalog []models.UserBook,
	meta ebookmeta.Metadata,
) *models.UserBook {
	if ub := matchLibraryByMetadata(catalog, meta); ub != nil {
		return ub
	}

	fileAuthors := make(map[string]struct{}, len(meta.Authors))
	for _, a := range meta.Authors {
		if n := normalizeAuthor(a); n != "" {
			fileAuthors[n] = struct{}{}
		}
	}
	if len(fileAuthors) == 0 {
		return nil
	}
	fileTokens := titleTokens(meta.Title)
	if len(fileTokens) == 0 {
		return nil
	}

	for i := range catalog {
		ub := &catalog[i]
		if ub.Book == nil {
			continue
		}
		if !titlesFuzzyMatch(fileTokens, titleTokens(ub.Book.Title)) {
			continue
		}
		for _, a := range ub.Book.Authors {
			if _, ok := fileAuthors[normalizeAuthor(a)]; ok {
				return ub
			}
		}
	}
	return nil
}
