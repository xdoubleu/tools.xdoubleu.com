package books

import (
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

type Clients struct {
	OpenLibrary openlibrary.Client
	GoogleBooks googlebooks.Client
	UniCat      unicat.Client
	Hardcover   hardcover.Client
	ObjectStore objectstore.Client
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
