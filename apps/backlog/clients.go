package backlog

import (
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
)

type Clients struct {
	SteamFactory func(apiKey string) steam.Client
	Goodreads    goodreads.Client
}
