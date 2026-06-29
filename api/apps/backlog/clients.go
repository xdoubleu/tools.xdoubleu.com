package backlog

import (
	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/apps/backlog/pkg/unicat"
)

type Clients struct {
	SteamFactory func(apiKey string) steam.Client
	OpenLibrary  openlibrary.Client
	GoogleBooks  googlebooks.Client
	UniCat       unicat.Client
	ObjectStore  objectstore.Client
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
