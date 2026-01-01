package goaltracker

import (
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
)

type Clients struct {
	Steam     steam.Client
	Todoist   todoist.Client
	Goodreads goodreads.Client
}
