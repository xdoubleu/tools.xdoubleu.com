package services

import (
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// maxCategoryLabelLen bounds a card label; longer text is prose, not a tag.
const maxCategoryLabelLen = 40

// categoryClassWords mark an element as a label when a '-'/'_'-separated
// word of one of its class tokens matches.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var categoryClassWords = map[string]bool{
	"tag": true, "tags": true, "category": true, "categories": true,
}

// labelSkippedTags never hold a card's labels: site chrome and filter forms.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var labelSkippedTags = map[string]bool{
	"nav": true, "header": true, "footer": true, "aside": true, "form": true,
	"script": true, "style": true,
}

// minTagListEntries is the fewest text-bearing children that make a label
// element a list of separate tags.
const minTagListEntries = 2

// multiplePostURLs marks a subtree holding more than one distinct post URL.
const multiplePostURLs = "\x00multiple"

// attachCardCategories sets each link's Categories from the labels on its
// cards. A card is the smallest ancestor of one of the link's anchors that
// holds labels and no other post URL; labels from every card showing the
// same post (e.g. grid and list views) are merged.
func attachCardCategories(
	doc *html.Node,
	base, localeBase *url.URL,
	links []discoveredLink,
) {
	posts := make(map[string]bool, len(links))
	for _, link := range links {
		posts[link.URL] = true
	}

	finder := cardFinder{
		base:       base,
		localeBase: localeBase,
		posts:      posts,
		owner:      map[*html.Node]string{},
		anchors:    nil,
	}
	finder.markPostURLs(doc)

	labels := make(map[string][]string, len(links))
	for _, a := range finder.anchors {
		labels[a.postURL] = append(labels[a.postURL], finder.cardLabels(a)...)
	}
	for i := range links {
		links[i].Categories = normalizeCategories(labels[links[i].URL])
	}
}

type cardFinder struct {
	base, localeBase *url.URL
	posts            map[string]bool
	// owner is the one post URL in a node's subtree, "" for none, or
	// multiplePostURLs.
	owner map[*html.Node]string
	// anchors are the post-link <a>s in DOM order.
	anchors []postAnchor
}

type postAnchor struct {
	node    *html.Node
	postURL string
}

// markPostURLs fills owner and anchors for n's subtree and returns owner[n].
// Site chrome is skipped, as in collectPostLinks.
func (f *cardFinder) markPostURLs(n *html.Node) string {
	if n.Type == html.ElementNode && skippedContainerTags[n.Data] {
		return ""
	}

	owned := ""
	if n.Type == html.ElementNode && n.Data == "a" {
		if resolved, ok := candidatePostURL(n, f.base, f.localeBase); ok &&
			f.posts[resolved.String()] {
			owned = resolved.String()
			f.anchors = append(f.anchors, postAnchor{node: n, postURL: owned})
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		owned = mergeOwner(owned, f.markPostURLs(c))
	}
	f.owner[n] = owned
	return owned
}

func mergeOwner(a, b string) string {
	switch {
	case a == "" || a == b:
		return b
	case b == "":
		return a
	default:
		return multiplePostURLs
	}
}

// cardLabels climbs from the anchor while the ancestor holds only its post
// URL, returning the labels of the first ancestor that has any.
func (f *cardFinder) cardLabels(a postAnchor) []string {
	for n := a.node; n != nil && n.Type == html.ElementNode; n = n.Parent {
		if n.Data == "body" || n.Data == "html" || f.owner[n] != a.postURL {
			break
		}
		if labels := f.labelsUnder(n); len(labels) > 0 {
			return labels
		}
	}
	return nil
}

// labelsUnder returns the text of the innermost label elements under n.
func (f *cardFinder) labelsUnder(n *html.Node) []string {
	if n.Type == html.ElementNode && labelSkippedTags[n.Data] {
		return nil
	}
	if f.isLabel(n) && !f.hasLabelDescendant(n) {
		return labelTexts(n)
	}

	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, f.labelsUnder(c)...)
	}
	return out
}

func (f *cardFinder) hasLabelDescendant(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if findNode(c, f.isLabel) != nil {
			return true
		}
	}
	return false
}

// isLabel is isCategoryLabel minus elements wrapping a post link or heading,
// which are cards carrying taxonomy classes (WordPress's category-news).
func (f *cardFinder) isLabel(n *html.Node) bool {
	return isCategoryLabel(n) && f.owner[n] == "" &&
		findNode(n, func(c *html.Node) bool {
			return c.Type == html.ElementNode && headingTags[c.Data]
		}) == nil
}

// labelTexts is one label per text-bearing child element when there are
// several (a tag list of plain entries), else the element's own text.
func labelTexts(n *html.Node) []string {
	var entries []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if text := collapsedText(c); text != "" {
			entries = append(entries, text)
		}
	}
	if len(entries) < minTagListEntries {
		entries = []string{collapsedText(n)}
	}

	var out []string
	for _, text := range entries {
		if text != "" && utf8.RuneCountInString(text) <= maxCategoryLabelLen {
			out = append(out, text)
		}
	}
	return out
}

func collapsedText(n *html.Node) string {
	return strings.Join(strings.Fields(nodeText(n)), " ")
}

// isCategoryLabel matches Finsweet's fs-list-field="category", rel="tag"
// anchors, and elements with a tag/category class word.
func isCategoryLabel(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if strings.EqualFold(nodeAttr(n, "fs-list-field"), "category") {
		return true
	}
	if n.Data == "a" && slices.Contains(
		strings.Fields(strings.ToLower(nodeAttr(n, "rel"))), "tag",
	) {
		return true
	}
	for _, token := range strings.Fields(strings.ToLower(nodeAttr(n, "class"))) {
		words := strings.FieldsFunc(token, func(r rune) bool {
			return r == '-' || r == '_'
		})
		if slices.ContainsFunc(words, func(w string) bool {
			return categoryClassWords[w]
		}) {
			return true
		}
	}
	return false
}
