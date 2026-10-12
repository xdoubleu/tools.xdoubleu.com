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
	// OnMyServices is set when a movie or series streams on one of the
	// user's services.
	OnMyServices bool
}

// SameTitle reports whether two titles name the same work, ignoring case,
// punctuation and a leading article. "Dune" and "Dune: Part Two" differ.
func SameTitle(a, b string) bool {
	na := normalizeTitle(a)
	return na != "" && na == normalizeTitle(b)
}

// BestTitle returns the index of the title in titles naming want: an exact
// SameTitle match, else one equal once a subtitle is dropped from either
// side ("Children of Time: A Novel"), else -1.
func BestTitle(titles []string, want string) int {
	for i, t := range titles {
		if SameTitle(t, want) {
			return i
		}
	}
	nw, mw := normalizeTitle(want), mainTitle(want)
	for i, t := range titles {
		if nw != "" && (mainTitle(t) == nw || normalizeTitle(t) == mw) {
			return i
		}
	}
	return -1
}

// mainTitle is the normalized title before a subtitle separator.
func mainTitle(title string) string {
	parts := strings.FieldsFunc(title, func(r rune) bool {
		return strings.ContainsRune(":(—–", r)
	})
	if len(parts) == 0 {
		return ""
	}
	return normalizeTitle(parts[0])
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
