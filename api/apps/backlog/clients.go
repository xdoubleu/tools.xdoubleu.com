package backlog

import (
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
)

type Clients struct {
	SteamFactory     func(apiKey string) steam.Client
	HardcoverFactory func(apiKey string) hardcover.Client
	ObjectStore      objectstore.Client
	// KoboStoreBaseURL is the upstream Kobo store base URL used for proxy/merge.
	// Override in tests to point at a stub upstream.
	KoboStoreBaseURL string
}
