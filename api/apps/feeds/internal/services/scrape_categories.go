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

// multiplePostURLs marks a subtree holding more than one distinct post URL.
const multiplePostURLs = "\x00multiple"

// attachCardCategories sets each link's Categories from the labels on its
// cards. A card is the highest ancestor below <body> of one of the link's
// anchors that holds no other post URL; labels from every card showing the
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
		card := finder.card(a.node, a.postURL)
		labels[a.postURL] = append(labels[a.postURL], cardLabels(card)...)
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

// card climbs from anchor while the ancestor holds only postURL.
func (f *cardFinder) card(anchor *html.Node, postURL string) *html.Node {
	card := anchor
	for p := anchor.Parent; p != nil && p.Type == html.ElementNode; p = p.Parent {
		if p.Data == "body" || p.Data == "html" || f.owner[p] != postURL {
			break
		}
		card = p
	}
	return card
}

// cardLabels returns the text of the innermost label elements under n.
func cardLabels(n *html.Node) []string {
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return nil
	}
	if isCategoryLabel(n) && !hasLabelDescendant(n) {
		text := strings.Join(strings.Fields(nodeText(n)), " ")
		if text == "" || utf8.RuneCountInString(text) > maxCategoryLabelLen {
			return nil
		}
		return []string{text}
	}

	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, cardLabels(c)...)
	}
	return out
}

func hasLabelDescendant(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if findNode(c, isCategoryLabel) != nil {
			return true
		}
	}
	return false
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
