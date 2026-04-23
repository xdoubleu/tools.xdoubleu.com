package backlog

import (
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
)

type Clients struct {
	SteamFactory     func(apiKey string) steam.Client
	HardcoverFactory func(apiKey string) hardcover.Client
}
