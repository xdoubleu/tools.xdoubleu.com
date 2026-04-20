package goaltracker

import (
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type Clients struct {
	SteamFactory   func(apiKey string) steam.Client
	TodoistFactory func(apiKey string) todoist.Client
	Goodreads      goodreads.Client
}
