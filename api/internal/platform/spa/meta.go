package spa

import (
	"bytes"
	"strings"
)

const defaultTitle = "<title>tools.xdoubleu.com</title>"

// sharedTitle names the public share pages under /dashboard/{games,reading}
// (owner dashboards are exactly /dashboard/games and /dashboard/reading).
// Token URLs are capability links, so they also get a noindex tag.
func sharedTitle(urlPath string) (string, bool) {
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	const tokenDepth = 3
	if len(parts) < tokenDepth || parts[0] != "dashboard" {
		return "", false
	}
	rest := parts[tokenDepth:]
	switch parts[1] {
	case "games":
		switch {
		case len(rest) == 0:
			return "Shared games", true
		case len(rest) == 1 && rest[0] == "library":
			return "Shared library", true
		case len(rest) == 1:
			return "Shared game", true
		}
	case "reading":
		switch {
		case len(rest) == 0:
			return "Shared reading dashboard", true
		case len(rest) == 1 && rest[0] == "library":
			return "Shared library", true
		}
	}
	return "", false
}

// indexFor is the HTML shell for a URL, with a share page's title and robots
// tag injected so crawlers and link previews see them without running JS.
func (h *Handler) indexFor(urlPath string) []byte {
	title, shared := sharedTitle(urlPath)
	if !shared {
		return h.index
	}

	h.variantsMu.Lock()
	defer h.variantsMu.Unlock()
	if body, ok := h.variants[title]; ok {
		return body
	}
	body := bytes.Replace(h.index, []byte(defaultTitle),
		[]byte("<title>"+htmlEscape(title)+"</title>\n    "+
			`<meta name="robots" content="noindex, nofollow" />`), 1)
	h.variants[title] = body
	return body
}

//nolint:gochecknoglobals //read-only
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;",
)

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }
