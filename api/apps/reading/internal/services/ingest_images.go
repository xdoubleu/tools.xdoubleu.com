package services

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	xhtml "golang.org/x/net/html"
)

const (
	// maxArticleImages caps how many images are embedded per article.
	maxArticleImages = 20
	// maxImageBytes caps a single downloaded image.
	maxImageBytes = int64(5 << 20)
)

// imageExtensions maps image content types to file extensions; anything else
// is skipped (Calibre needs an extension it recognizes).
//
//nolint:gochecknoglobals // static lookup table
var imageExtensions = map[string]string{
	"image/jpeg":    ".jpg",
	"image/png":     ".png",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",
}

// localizeImages downloads the article's <img> targets into dir and rewrites
// their src attributes to the local file names, so the EPUB conversion embeds
// them without any network access. Images that fail to download (or exceed
// caps) are stripped rather than failing the whole build. Any parse failure
// falls back to the original HTML unchanged.
func (s *IngestService) localizeImages(
	ctx context.Context,
	dir, docHTML, baseURL string,
) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return docHTML
	}

	root, err := xhtml.Parse(strings.NewReader(docHTML))
	if err != nil {
		return docHTML
	}

	count := 0
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == xhtml.ElementNode && child.Data == "img" {
				if !s.localizeImg(ctx, dir, base, child, &count) {
					n.RemoveChild(child)
				}
			} else {
				walk(child)
			}
			child = next
		}
	}
	walk(root)

	var out strings.Builder
	if err = xhtml.Render(&out, root); err != nil {
		return docHTML
	}
	return out.String()
}

// localizeImg downloads one img node's target and rewrites its src. Returns
// false when the node should be stripped.
func (s *IngestService) localizeImg(
	ctx context.Context,
	dir string,
	base *url.URL,
	node *xhtml.Node,
	count *int,
) bool {
	src := ""
	for _, attr := range node.Attr {
		if attr.Key == "src" {
			src = attr.Val
			break
		}
	}
	if src == "" || *count >= maxArticleImages {
		return false
	}

	resolved, err := base.Parse(src)
	if err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
		return false
	}

	res, err := s.webFetch.Get(
		ctx, resolved.String(), fetchOptions(maxImageBytes, "image/*"),
	)
	if err != nil {
		s.logger.DebugContext(ctx, "skipping article image",
			"src", resolved.String(), "error", err)
		return false
	}
	ext, ok := imageExtensions[res.ContentType]
	if !ok {
		return false
	}

	name := fmt.Sprintf("img_%d%s", *count, ext)
	//nolint:gosec // temp file, no sensitive contents
	if err = os.WriteFile(filepath.Join(dir, name), res.Body, 0o644); err != nil {
		return false
	}
	*count++

	for i := range node.Attr {
		if node.Attr[i].Key == "src" {
			node.Attr[i].Val = name
		}
	}
	// srcset would override the rewritten src with remote URLs; drop it.
	node.Attr = removeAttr(node.Attr, "srcset")
	return true
}

func removeAttr(attrs []xhtml.Attribute, key string) []xhtml.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key != key {
			out = append(out, a)
		}
	}
	return out
}
