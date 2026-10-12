package consumption

import (
	"strings"
	"unicode"
)

// CatalogMatch is a recommendation candidate found in its domain's catalog.
type CatalogMatch struct {
	// Ref is the owning app's catalog id, e.g. "movie:603" or a Steam app id.
	Ref      string
	Title    string
	Creator  string
	Year     int
	ImageURL string
	URL      string
	// Known is set when the item is already in the user's library, backlog,
	// favourites or feeds.
	Known bool
}

// SameTitle reports whether two titles name the same work, ignoring case,
// punctuation, a leading article and a subtitle on either side.
func SameTitle(a, b string) bool {
	na, nb := normalizeTitle(a), normalizeTitle(b)
	if na == "" || nb == "" {
		return false
	}
	return na == nb || mainTitle(a) == nb || na == mainTitle(b)
}

// mainTitle is the normalized title before a subtitle separator.
func mainTitle(title string) string {
	if i := strings.IndexAny(title, ":(—–"); i > 0 {
		return normalizeTitle(title[:i])
	}
	return normalizeTitle(title)
}

func normalizeTitle(title string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		case r == '\'' || r == '’':
		default:
			space = true
		}
	}
	out := b.String()
	for _, article := range []string{"the ", "a ", "an "} {
		if rest, ok := strings.CutPrefix(out, article); ok && rest != "" {
			return rest
		}
	}
	return out
}
