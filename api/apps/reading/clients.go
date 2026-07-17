package reading

import (
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/hardcover"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/unicat"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

type Clients struct {
	UniCat      unicat.Client
	Hardcover   hardcover.Client
	ObjectStore objectstore.Client
	// WebFetch fetches external web content (articles, RSS bodies, PDFs,
	// images) with size caps and conditional-GET support.
	WebFetch webfetch.Client
	// Arxiv resolves arXiv ids to paper metadata.
	Arxiv arxiv.Client
	// HTMLConvert overrides the article HTML→EPUB converter. Leave nil for
	// the default (Calibre ebook-convert subprocess); tests inject a fake
	// since the binary is not available there.
	HTMLConvert services.HTMLConverter
	// KoboStoreBaseURL is the upstream Kobo store base URL used for proxy/merge.
	// Override in tests to point at a stub upstream.
	KoboStoreBaseURL string
	// PublicAPIBaseURL is the externally reachable base URL of this API server
	// (e.g. "https://tools.xdoubleu.com/api"). It is prepended to the library
	// path when building Kobo file download URLs so the device can reach the
	// file handler even when a reverse proxy strips a path prefix before
	// forwarding. Leave empty in tests; koboLibraryBase falls back to
	// request-header derivation.
	PublicAPIBaseURL string
}
